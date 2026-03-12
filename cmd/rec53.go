package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rec53/monitor"
	"rec53/server"

	"gopkg.in/yaml.v2"
)

var (
	configPath  = flag.String("config", "", "Path to YAML config file (required)")
	noWarmup    = flag.Bool("no-warmup", false, "Disable NS warmup on startup")
	listenAddr  = flag.String("listen", "127.0.0.1:5353", "DNS server listen address (host:port)")
	metricAddr  = flag.String("metric", ":9999", "Prometheus metrics listen address (host:port)")
	logLevel    = flag.String("log-level", "info", "Log level: debug, info, warn, error")
	showVersion = flag.Bool("version", false, "Show version information")
)

// Config represents the overall application configuration
type Config struct {
	DNS    DNSConfig           `yaml:"dns"`
	Warmup server.WarmupConfig `yaml:"warmup"`
}

// DNSConfig represents DNS server configuration
type DNSConfig struct {
	Listen   string `yaml:"listen"`
	Metric   string `yaml:"metric"`
	LogLevel string `yaml:"log_level"`
}

// loadConfig loads configuration from a YAML file.
// Returns error if config is required (--config flag) but not provided.
func loadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		return nil, fmt.Errorf(
			"Config file required.\nGenerate default config with:\n" +
				"  ./generate-config.sh\n" +
				"  ./rec53 --config ./config.yaml",
		)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf(
			"Config file not found: %s\n"+
				"Generate it with:\n"+
				"  ./generate-config.sh\n"+
				"  ./rec53 --config %s",
			configPath, configPath,
		)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("Failed to parse config: %v", err)
	}

	return &cfg, nil
}

// Version information (set via ldflags during build)
var (
	version   = "dev"
	buildTime = "unknown"
)

// shutdownFunc is a function that can be shut down with a context
type shutdownFunc func(ctx context.Context) error

// gracefulShutdown shuts down multiple components with a timeout context.
// It logs errors but does not return them, making it suitable for defer or cleanup.
func gracefulShutdown(ctx context.Context, shutdowns ...shutdownFunc) {
	for _, shutdown := range shutdowns {
		if shutdown != nil {
			if err := shutdown(ctx); err != nil {
				monitor.Rec53Log.Errorf("Shutdown error: %s", err.Error())
			}
		}
	}
}

// waitForSignal blocks until a signal is received or the server errors.
// It returns the signal that was received, or nil if the server errored.
func waitForSignal(sigChan chan os.Signal, errChan <-chan error) os.Signal {
	select {
	case err := <-errChan:
		if err != nil {
			monitor.Rec53Log.Errorf("Server error: %s", err.Error())
		}
		return nil
	case s := <-sigChan:
		monitor.Rec53Log.Infof("Signal (%v) received, shutting down gracefully", s)
		return s
	}
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("rec53 version: %s\nbuilt: %s\n", version, buildTime)
		return
	}

	// Load configuration
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	// Override config with command-line flags if provided
	// (Command-line flags take precedence over config file)
	if listenAddr != nil && *listenAddr != "" && *listenAddr != "127.0.0.1:5353" {
		cfg.DNS.Listen = *listenAddr
	}
	if metricAddr != nil && *metricAddr != "" && *metricAddr != ":9999" {
		cfg.DNS.Metric = *metricAddr
	}
	if logLevel != nil && *logLevel != "" && *logLevel != "info" {
		cfg.DNS.LogLevel = *logLevel
	}

	// Handle --no-warmup flag
	if *noWarmup {
		cfg.Warmup.Enabled = false
	}

	// Initialize logger
	monitor.InitLogger()
	defer monitor.Rec53Log.Sync()
	monitor.SetLogLevel(parseLogLevel(cfg.DNS.LogLevel).Level())

	// Initialize metrics server
	monitor.InitMetricWithAddr(cfg.DNS.Metric)

	// Start DNS server with config
	rec53 := server.NewServerWithConfig(cfg.DNS.Listen, cfg.Warmup)
	errChan := rec53.Run()

	monitor.Rec53Log.Infof("rec53 started, listening on %s, metrics on %s", cfg.DNS.Listen, cfg.DNS.Metric)

	// Wait for shutdown signal or server error
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	waitForSignal(sig, errChan)

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gracefulShutdown(ctx, rec53.Shutdown, monitor.ShutdownMetric)

	monitor.Rec53Log.Info("rec53 stopped")
}
