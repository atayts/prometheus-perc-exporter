package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the on-disk configuration of the exporter. Every runtime option
// lives here so that changing one only means editing the file and restarting
// the service — no reinstall and no service binPath surgery.
type Config struct {
	PerccliPath    string `yaml:"perccli_path"`
	ListenAddress  string `yaml:"web_listen_address"`
	ScrapeInterval int    `yaml:"scrape_interval"`
	LogFile        string `yaml:"log_file"`
}

const (
	// configFileName is looked up next to the executable when no path is given.
	configFileName = "config.yml"
	// maxLogSize is the size at which the log file is rotated to <name>.old.
	maxLogSize = 10 << 20
)

func defaultConfig() *Config {
	return &Config{
		PerccliPath:    `C:\ProgramData\chocolatey\bin\perccli64.exe`,
		ListenAddress:  ":9102",
		ScrapeInterval: 900,
	}
}

// executableDir returns the directory holding the running binary, which is
// where the service looks for its configuration and writes its log.
func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// configFilePath resolves the configuration file to read. An empty flagPath
// means "config.yml next to the executable".
func configFilePath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}
	return filepath.Join(executableDir(), configFileName)
}

// defaultLogPath is where a service logs when the config names no log file.
func defaultLogPath() string {
	return filepath.Join(executableDir(), serviceName+".log")
}

// loadConfig layers the file on top of the built-in defaults: a key absent from
// the file keeps its default, so a partial configuration file is valid. A
// missing file is only an error when the path was requested explicitly.
func loadConfig(path string, required bool) (*Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !required {
			log.Printf("No configuration file at %s, using built-in defaults", path)
			return cfg, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	log.Printf("Loaded configuration from %s", path)
	return cfg, nil
}

func (c *Config) validate() error {
	if c.PerccliPath == "" {
		return errors.New("perccli_path must not be empty")
	}
	if c.ListenAddress == "" {
		return errors.New("web_listen_address must not be empty")
	}
	if c.ScrapeInterval <= 0 {
		return fmt.Errorf("scrape_interval must be greater than 0, got %d", c.ScrapeInterval)
	}
	return nil
}

// logFile is the currently open log destination, kept so a second
// setupLogging call can close it.
var logFile *os.File

// setupLogging tees the log to path, rotating one generation when it grows too
// large. A Windows service has nowhere to write stderr, so without this a bad
// configuration file would make the service fail to start with no explanation.
func setupLogging(path string) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("WARNING: cannot create log directory for %s: %v", path, err)
		return
	}
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxLogSize {
		os.Remove(path + ".old")
		os.Rename(path, path+".old")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("WARNING: cannot open log file %s: %v", path, err)
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	if logFile != nil {
		logFile.Close()
	}
	logFile = f
}
