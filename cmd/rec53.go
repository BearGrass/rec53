package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
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

	// Apply TLD list configuration: if custom TLDs are provided, use them; otherwise use curated defaults
	cfg.Warmup.TLDs = server.LoadTLDList(cfg.Warmup.TLDs)

	// Apply warmup concurrency configuration:
	// If not explicitly set in config (concurrency == 0), use the dynamically calculated default.
	// If explicitly set, respect the user's value.
	if cfg.Warmup.Concurrency == 0 {
		cfg.Warmup.Concurrency = server.DefaultWarmupConfig.Concurrency
	}

	return &cfg, nil
}

// validateConfig validates critical configuration fields before use.
// Returns error if configuration is invalid.
func validateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Validate listen address
	if strings.TrimSpace(cfg.DNS.Listen) == "" {
		return fmt.Errorf("dns.listen address is required and cannot be empty")
	}

	// Validate metric address
	if strings.TrimSpace(cfg.DNS.Metric) == "" {
		return fmt.Errorf("dns.metric address is required and cannot be empty")
	}

	// Validate listen address can be parsed
	if _, err := net.ResolveTCPAddr("tcp", cfg.DNS.Listen); err != nil {
		return fmt.Errorf("invalid dns.listen address '%s': %v", cfg.DNS.Listen, err)
	}

	// Validate metric address can be parsed (allow just port)
	metricStr := strings.TrimSpace(cfg.DNS.Metric)
	if metricStr[0] == ':' {
		// Port-only format
		portStr := metricStr[1:]
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("invalid dns.metric port '%s': %v", cfg.DNS.Metric, err)
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("dns.metric port must be between 1 and 65535, got %d", port)
		}
	} else {
		// Full address format
		if _, err := net.ResolveTCPAddr("tcp", cfg.DNS.Metric); err != nil {
			return fmt.Errorf("invalid dns.metric address '%s': %v", cfg.DNS.Metric, err)
		}
	}

	// Validate warmup config exists
	if cfg.Warmup.Timeout > 0 && cfg.Warmup.Timeout < 100*time.Millisecond {
		return fmt.Errorf("warmup.timeout must be at least 100ms, got %v", cfg.Warmup.Timeout)
	}

	return nil
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
	// Recover from panics during startup and log them
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "FATAL: Panic during startup: %v\n", r)
			fmt.Fprintf(os.Stderr, "Stack trace:\n%s\n", debug.Stack())
			os.Exit(1)
		}
	}()

	flag.Parse()

	if *showVersion {
		fmt.Printf("rec53 version: %s\nbuilt: %s\n", version, buildTime)
		return
	}

	// Load configuration
	fmt.Fprintf(os.Stderr, "DEBUG: Loading config from %s\n", *configPath)
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "DEBUG: Config loaded successfully\n")

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

	// Validate configuration before using it
	fmt.Fprintf(os.Stderr, "DEBUG: Validating configuration\n")
	if err := validateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %s\n", err.Error())
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "DEBUG: Configuration validation passed\n")

	// Initialize logger
	fmt.Fprintf(os.Stderr, "DEBUG: Initializing logger\n")
	monitor.InitLogger()
	defer monitor.Rec53Log.Sync()
	monitor.SetLogLevel(parseLogLevel(cfg.DNS.LogLevel).Level())
	monitor.Rec53Log.Debugf("Logger initialized with level: %s", cfg.DNS.LogLevel)

	// Initialize metrics server with error handling
	fmt.Fprintf(os.Stderr, "DEBUG: Initializing metrics server on %s\n", cfg.DNS.Metric)
	monitor.InitMetricWithAddr(cfg.DNS.Metric)
	monitor.Rec53Log.Debugf("Metrics server initialized on %s", cfg.DNS.Metric)

	// Start DNS server with config and panic recovery
	defer func() {
		if r := recover(); r != nil {
			monitor.Rec53Log.Errorf("PANIC during server startup: %v", r)
			monitor.Rec53Log.Errorf("Stack trace: %s", debug.Stack())
			os.Exit(1)
		}
	}()

	fmt.Fprintf(os.Stderr, "DEBUG: Creating DNS server with listen address %s\n", cfg.DNS.Listen)
	rec53 := server.NewServerWithConfig(cfg.DNS.Listen, cfg.Warmup)
	monitor.Rec53Log.Debugf("DNS server created, starting...")
	errChan := rec53.Run()
	monitor.Rec53Log.Debugf("DNS server started")

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
