package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/alecthomas/kingpin/v2"
	"golang.org/x/sys/windows/svc"
)

func main() {
	perccliPath := kingpin.Flag("perccli.path", "Path to perccli64 executable.").
		Default(`C:\ProgramData\chocolatey\bin\perccli64.exe`).String()
	listenAddr := kingpin.Flag("web.listen-address", "Address to listen on for metrics.").
		Default(":9102").String()
	scrapeInterval := kingpin.Flag("scrape.interval", "How often to scrape PERC data in seconds.").
		Default("900").Int()

	kingpin.HelpFlag.Short('h')
	kingpin.Version(version)
	kingpin.Parse()

	log.Printf("Starting PERC Windows Exporter v%s", version)

	p := exporterParams{
		PerccliPath:    *perccliPath,
		ListenAddr:     *listenAddr,
		ScrapeInterval: *scrapeInterval,
	}

	isService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("Failed to detect service mode: %v", err)
	}

	if isService {
		log.Println("Running as Windows Service")
		runAsService(p)
	} else {
		log.Println("Running in console mode (Ctrl+C to stop)")
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		runExporter(ctx, p)
	}
}
