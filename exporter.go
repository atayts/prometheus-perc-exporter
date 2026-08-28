package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const version = "1.1.0"

var (
	controllerOKStatuses = []string{"optimal"}
	vdOKStates           = []string{"optl"}
	pdOKStates           = []string{"onln", "ugood", "dhs", "ghs"}
)

var (
	percControllerStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "perc_controller_status",
		Help: "PERC controller status (0=ok, 1=degraded, 2=failed).",
	}, []string{"controller", "model"})

	percBBUStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "perc_bbu_status",
		Help: "Battery backup unit status (0=ok, 1=warning, 2=critical).",
	}, []string{"controller"})

	percVDStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "perc_virtual_disk_status",
		Help: "Virtual disk status (0=optimal, 1=degraded, 2=failed).",
	}, []string{"controller", "vd"})

	percPDStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "perc_physical_disk_status",
		Help: "Physical disk status (0=ok, 1=degraded, 2=failed).",
	}, []string{"controller", "enclosure", "slot"})

	percPDMediaErrors = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "perc_physical_disk_media_errors_total",
		Help: "Physical disk media error count.",
	}, []string{"controller", "enclosure", "slot"})

	percPDOtherErrors = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "perc_physical_disk_other_errors_total",
		Help: "Physical disk other error count.",
	}, []string{"controller", "enclosure", "slot"})

	percPDPredictiveFailures = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "perc_physical_disk_predictive_failures_total",
		Help: "Physical disk predictive failure count.",
	}, []string{"controller", "enclosure", "slot"})

	percPDSmartAlert = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "perc_physical_disk_smart_alert",
		Help: "Physical disk SMART alert status.",
	}, []string{"controller", "enclosure", "slot"})

	percScrapeErrors = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "perc_scrape_errors_total",
		Help: "Number of errors scraping PERC data.",
	})

	percExporterInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "perc_exporter_info",
		Help: "PERC exporter information.",
	}, []string{"version"})
)

func init() {
	prometheus.MustRegister(
		percControllerStatus, percBBUStatus,
		percVDStatus,
		percPDStatus, percPDMediaErrors, percPDOtherErrors,
		percPDPredictiveFailures, percPDSmartAlert,
		percScrapeErrors, percExporterInfo,
	)
}

func runPerccli(perccliPath string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, perccliPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("perccli command timed out")
		}
		return nil, fmt.Errorf("perccli failed: %w", err)
	}
	return out, nil
}

func runPerccliJSON(perccliPath string, args ...string) (map[string]interface{}, error) {
	args = append(args, "J")
	out, err := runPerccli(perccliPath, args...)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parsing perccli JSON: %w", err)
	}
	return result, nil
}

func jsonStr(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func jsonInt(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 0
	}
}

func jsonMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func jsonArr(v interface{}) []interface{} {
	if a, ok := v.([]interface{}); ok {
		return a
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

type controllerInfo struct {
	ID     int
	Model  string
	Status string
}

type bbuInfo struct {
	Controller int
	Bad        bool
	Replace    bool
}

type vdInfo struct {
	Controller int
	VD         int
	State      string
}

type pdInfo struct {
	Controller  int
	Enclosure   string
	Slot        string
	State       string
	MediaErr    int
	OtherErr    int
	PredictFail int
	SmartAlert  int
}

type percData struct {
	Controllers []controllerInfo
	BBUs        []bbuInfo
	VDs         []vdInfo
	PDs         []pdInfo
}

func getInfo(perccliPath string) (*percData, error) {
	data := &percData{}

	ctrlJSON, err := runPerccliJSON(perccliPath, "/call", "show", "all")
	if err != nil {
		return nil, fmt.Errorf("reading controller info: %w", err)
	}

	controllers := jsonArr(ctrlJSON["Controllers"])
	for _, c := range controllers {
		ctrl := jsonMap(c)
		resp := jsonMap(ctrl["Response Data"])
		if resp == nil {
			continue
		}

		basics := jsonMap(resp["Basics"])
		status := jsonMap(resp["Status"])
		hwCfg := jsonMap(resp["HwCfg"])
		ctrlID := jsonInt(basics["Controller"])
		ctrlIDStr := strconv.Itoa(ctrlID)

		data.Controllers = append(data.Controllers, controllerInfo{
			ID:     ctrlID,
			Model:  jsonStr(basics["Model"]),
			Status: strings.ToLower(jsonStr(status["Controller Status"])),
		})

		// Battery backup unit.
		if jsonStr(hwCfg["BBU"]) != "Absent" {
			bbuJSON, err := runPerccliJSON(perccliPath, "/c"+ctrlIDStr+"/bbu", "show", "status")
			if err != nil {
				return nil, fmt.Errorf("reading BBU info: %w", err)
			}
			bbuControllers := jsonArr(bbuJSON["Controllers"])
			if ctrlID < len(bbuControllers) {
				bbuCtrl := jsonMap(bbuControllers[ctrlID])
				bbuResp := jsonMap(bbuCtrl["Response Data"])

				gasGauge := make(map[string]string)
				for _, item := range jsonArr(bbuResp["GasGaugeStatus"]) {
					m := jsonMap(item)
					gasGauge[jsonStr(m["Property"])] = jsonStr(m["Value"])
				}
				bbuFW := make(map[string]string)
				for _, item := range jsonArr(bbuResp["BBU_Firmware_Status"]) {
					m := jsonMap(item)
					bbuFW[jsonStr(m["Property"])] = jsonStr(m["Value"])
				}

				data.BBUs = append(data.BBUs, bbuInfo{
					Controller: ctrlID,
					Bad:        gasGauge["Is SOH Good"] != "Yes",
					Replace:    bbuFW["Pack is about to fail & should be replaced"] != "No",
				})
			}
		}

		// Virtual disks.
		for _, v := range jsonArr(resp["VD LIST"]) {
			vd := jsonMap(v)
			parts := strings.Split(jsonStr(vd["DG/VD"]), "/")
			if len(parts) != 2 {
				continue
			}
			vdID, _ := strconv.Atoi(parts[1])
			data.VDs = append(data.VDs, vdInfo{
				Controller: ctrlID,
				VD:         vdID,
				State:      strings.ToLower(jsonStr(vd["State"])),
			})
		}

		// Detect enclosures.
		enclJSON, err := runPerccliJSON(perccliPath, "/c"+ctrlIDStr+"/eall", "show")
		if err != nil {
			return nil, fmt.Errorf("reading enclosure info: %w", err)
		}
		encl := ""
		enclControllers := jsonArr(enclJSON["Controllers"])
		if ctrlID < len(enclControllers) {
			enclCtrl := jsonMap(enclControllers[ctrlID])
			if jsonMap(enclCtrl["Response Data"]) != nil {
				encl = "/eall"
			}
		}

		// Physical disks.
		pdJSON, err := runPerccliJSON(perccliPath, "/c"+ctrlIDStr+encl+"/sall", "show", "all")
		if err != nil {
			return nil, fmt.Errorf("reading physical disk info: %w", err)
		}

		pdControllers := jsonArr(pdJSON["Controllers"])
		if ctrlID >= len(pdControllers) {
			continue
		}
		pdResp := jsonMap(jsonMap(pdControllers[ctrlID])["Response Data"])
		if pdResp == nil {
			continue
		}

		for _, p := range jsonArr(resp["PD LIST"]) {
			pd := jsonMap(p)
			eidSlt := jsonStr(pd["EID:Slt"])
			parts := strings.Split(eidSlt, ":")
			if len(parts) != 2 {
				continue
			}
			enc := strings.TrimSpace(parts[0])
			slot := strings.TrimSpace(parts[1])

			var pdIDKey, pdStateKey string
			if enc == "-" || enc == " " || enc == "" {
				pdIDKey = fmt.Sprintf("Drive /c%d/s%s - Detailed Information", ctrlID, slot)
				pdStateKey = fmt.Sprintf("Drive /c%d/s%s State", ctrlID, slot)
			} else {
				pdIDKey = fmt.Sprintf("Drive /c%d/e%s/s%s - Detailed Information", ctrlID, enc, slot)
				pdStateKey = fmt.Sprintf("Drive /c%d/e%s/s%s State", ctrlID, enc, slot)
			}

			detailedInfo := jsonMap(pdResp[pdIDKey])
			pdState := jsonMap(detailedInfo[pdStateKey])

			smartVal := 0
			if jsonStr(pdState["S.M.A.R.T alert flagged by drive"]) != "No" {
				smartVal = 1
			}

			data.PDs = append(data.PDs, pdInfo{
				Controller:  ctrlID,
				Enclosure:   enc,
				Slot:        slot,
				State:       strings.ToLower(jsonStr(pd["State"])),
				MediaErr:    jsonInt(pdState["Media Error Count"]),
				OtherErr:    jsonInt(pdState["Other Error Count"]),
				PredictFail: jsonInt(pdState["Predictive Failure Count"]),
				SmartAlert:  smartVal,
			})
		}
	}

	return data, nil
}

func collectMetrics(perccliPath string) {
	data, err := getInfo(perccliPath)
	if err != nil {
		log.Printf("ERROR: collecting PERC data: %v", err)
		percScrapeErrors.Inc()
		return
	}

	for _, ctrl := range data.Controllers {
		id := strconv.Itoa(ctrl.ID)
		val := 0.0
		if !contains(controllerOKStatuses, ctrl.Status) {
			val = 2
			log.Printf("WARNING: Controller %s (ID %s) status: %s", ctrl.Model, id, ctrl.Status)
		}
		percControllerStatus.WithLabelValues(id, ctrl.Model).Set(val)
	}

	for _, bbu := range data.BBUs {
		id := strconv.Itoa(bbu.Controller)
		var val float64
		if bbu.Bad {
			val = 2
		} else if bbu.Replace {
			val = 1
		}
		if val != 0 {
			state := "warning"
			if bbu.Bad {
				state = "failed"
			}
			log.Printf("WARNING: Controller %s BBU status: %s", id, state)
		}
		percBBUStatus.WithLabelValues(id).Set(val)
	}

	for _, vd := range data.VDs {
		ctrlID := strconv.Itoa(vd.Controller)
		vdID := strconv.Itoa(vd.VD)
		val := 0.0
		if !contains(vdOKStates, vd.State) {
			val = 2
			log.Printf("WARNING: Virtual disk %s on controller %s state: %s", vdID, ctrlID, vd.State)
		}
		percVDStatus.WithLabelValues(ctrlID, vdID).Set(val)
	}

	for _, pd := range data.PDs {
		ctrlID := strconv.Itoa(pd.Controller)
		labels := prometheus.Labels{
			"controller": ctrlID,
			"enclosure":  pd.Enclosure,
			"slot":       pd.Slot,
		}
		val := 0.0
		if !contains(pdOKStates, pd.State) {
			val = 2
			log.Printf("WARNING: Physical disk e%s:s%s on controller %s state: %s", pd.Enclosure, pd.Slot, ctrlID, pd.State)
		}
		if pd.SmartAlert != 0 {
			log.Printf("WARNING: Physical disk e%s:s%s on controller %s has SMART alert", pd.Enclosure, pd.Slot, ctrlID)
		}
		percPDStatus.With(labels).Set(val)
		percPDMediaErrors.With(labels).Set(float64(pd.MediaErr))
		percPDOtherErrors.With(labels).Set(float64(pd.OtherErr))
		percPDPredictiveFailures.With(labels).Set(float64(pd.PredictFail))
		percPDSmartAlert.With(labels).Set(float64(pd.SmartAlert))
	}

	percExporterInfo.WithLabelValues(version).Set(1)
	percScrapeErrors.Set(0)
}

func runExporter(ctx context.Context, cfg *Config) {
	log.Printf("Config: listen=%s scrape_interval=%ds perccli=%s",
		cfg.ListenAddress, cfg.ScrapeInterval, cfg.PerccliPath)

	log.Println("Performing initial PERC data collection...")
	collectMetrics(cfg.PerccliPath)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Duration(cfg.ScrapeInterval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				collectMetrics(cfg.PerccliPath)
			case <-ctx.Done():
				log.Println("Shutting down collection loop")
				return
			}
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body><h1>PERC Windows Exporter</h1><p><a href="/metrics">Metrics</a></p></body></html>`)
	})

	srv := &http.Server{Addr: cfg.ListenAddress, Handler: mux}

	go func() {
		<-ctx.Done()
		log.Println("Shutting down HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("Listening on %s", cfg.ListenAddress)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}

	wg.Wait()
	log.Println("Exporter stopped")
}
