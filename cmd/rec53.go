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
	DNS        DNSConfig             `yaml:"dns"`
	Warmup     server.WarmupConfig   `yaml:"warmup"`
	Snapshot   server.SnapshotConfig `yaml:"snapshot"`
	Hosts      []server.HostEntry    `yaml:"hosts"`
	Forwarding []server.ForwardZone  `yaml:"forwarding"`
	Debug      DebugConfig           `yaml:"debug"`
}

// DebugConfig holds debug/profiling configuration
type DebugConfig struct {
	PprofEnabled bool   `yaml:"pprof_enabled"`
	PprofListen  string `yaml:"pprof_listen"`
}

// DNSConfig represents DNS server configuration
type DNSConfig struct {
	Listen          string        `yaml:"listen"`
	Metric          string        `yaml:"metric"`
	LogLevel        string        `yaml:"log_level"`
	UpstreamTimeout time.Duration `yaml:"upstream_timeout"`
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
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	// Apply TLD list configuration: if custom TLDs are provided, use them; otherwise use curated defaults
	cfg.Warmup.TLDs = server.LoadTLDList(cfg.Warmup.TLDs)

	// Apply debug defaults: pprof listen address defaults to 127.0.0.1:6060 if not set
	if cfg.Debug.PprofListen == "" {
		cfg.Debug.PprofListen = "127.0.0.1:6060"
	}

	// Apply warmup concurrency configuration:
	// If not explicitly set in config (concurrency == 0), use the dynamically calculated default.
	// If explicitly set, respect the user's value.
	if cfg.Warmup.Concurrency == 0 {
		cfg.Warmup.Concurrency = server.DefaultWarmupConfig.Concurrency
	}

	// Apply upstream timeout: if not set (0), keep the server package default (1.5s).
	// If explicitly set, it will be applied via server.SetUpstreamTimeout after validation.

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

	// Validate upstream timeout: if set, must be at least 100ms
	if cfg.DNS.UpstreamTimeout > 0 && cfg.DNS.UpstreamTimeout < 100*time.Millisecond {
		return fmt.Errorf("dns.upstream_timeout must be at least 100ms, got %v", cfg.DNS.UpstreamTimeout)
	}

	// Validate hosts entries
	if err := validateHostsConfig(cfg.Hosts); err != nil {
		return err
	}

	// Validate forwarding entries
	if err := validateForwardingConfig(cfg.Forwarding); err != nil {
		return err
	}

	return nil
}

// validateHostsConfig validates all hosts entries in the configuration.
// Supported record types: A, AAAA, CNAME.
func validateHostsConfig(hosts []server.HostEntry) error {
	for i, h := range hosts {
		if strings.TrimSpace(h.Name) == "" {
			return fmt.Errorf("hosts[%d]: name must not be empty", i)
		}
		if strings.TrimSpace(h.Value) == "" {
			return fmt.Errorf("hosts[%d] (%s): value must not be empty", i, h.Name)
		}
		switch strings.ToUpper(h.Type) {
		case "A":
			ip := net.ParseIP(h.Value)
			if ip == nil || ip.To4() == nil {
				return fmt.Errorf("hosts[%d] (%s): invalid IPv4 address %q for A record", i, h.Name, h.Value)
			}
		case "AAAA":
			ip := net.ParseIP(h.Value)
			if ip == nil || ip.To4() != nil {
				return fmt.Errorf("hosts[%d] (%s): invalid IPv6 address %q for AAAA record", i, h.Name, h.Value)
			}
		case "CNAME":
			// CNAME value is a domain name; basic non-empty check is sufficient here.
		default:
			return fmt.Errorf("hosts[%d] (%s): unsupported record type %q (supported: A, AAAA, CNAME)", i, h.Name, h.Type)
		}
	}
	return nil
}

// validateForwardingConfig validates all forwarding zone entries in the configuration.
func validateForwardingConfig(zones []server.ForwardZone) error {
	for i, z := range zones {
		if strings.TrimSpace(z.Zone) == "" {
			return fmt.Errorf("forwarding[%d]: zone must not be empty", i)
		}
		if len(z.Upstreams) == 0 {
			return fmt.Errorf("forwarding[%d] (%s): upstreams list must not be empty", i, z.Zone)
		}
		for j, up := range z.Upstreams {
			if _, _, err := net.SplitHostPort(up); err != nil {
				return fmt.Errorf("forwarding[%d] (%s) upstream[%d]: invalid address %q: %v", i, z.Zone, j, up, err)
			}
		}
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
	// Recover from panics during startup and log them with a full stack trace.
	// This runs before the logger is initialized, so output goes to stderr.
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

	// Load configuration from file.
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	// Override config with command-line flags if provided.
	// Command-line flags take precedence over config file values.
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

	// Validate configuration before using any config values.
	if err := validateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %s\n", err.Error())
		os.Exit(1)
	}

	// Apply upstream timeout from config (0 means use the default 1.5s in the server package).
	if cfg.DNS.UpstreamTimeout > 0 {
		server.SetUpstreamTimeout(cfg.DNS.UpstreamTimeout)
	}

	// Initialize logger. All subsequent log output uses monitor.Rec53Log.
	monitor.InitLogger()
	defer monitor.Rec53Log.Sync()
	monitor.SetLogLevel(parseLogLevel(cfg.DNS.LogLevel).Level())
	monitor.Rec53Log.Debugf("logger initialized with level: %s", cfg.DNS.LogLevel)

	// Initialize Prometheus metrics server.
	monitor.InitMetricWithAddr(cfg.DNS.Metric)
	monitor.Rec53Log.Debugf("metrics server initialized on %s", cfg.DNS.Metric)

	// Start pprof server if enabled (default: off, listen on 127.0.0.1:6060).
	// Uses a dedicated context cancelled on shutdown signal.
	pprofCancel := func() {} // no-op if pprof not enabled
	if cfg.Debug.PprofEnabled {
		var pprofCtx context.Context
		pprofCtx, pprofCancel = context.WithCancel(context.Background())
		monitor.StartPprofServer(pprofCtx, cfg.Debug.PprofListen)
		monitor.Rec53Log.Infof("pprof server started on %s", cfg.Debug.PprofListen)
	}

	// Second panic recovery layer — after logger is available, panics are logged
	// via monitor.Rec53Log so they appear in the configured log output.
	defer func() {
		if r := recover(); r != nil {
			monitor.Rec53Log.Errorf("PANIC during server startup: %v", r)
			monitor.Rec53Log.Errorf("Stack trace: %s", debug.Stack())
			os.Exit(1)
		}
	}()

	// Create and start the DNS server.
	monitor.Rec53Log.Debugf("creating DNS server on %s", cfg.DNS.Listen)
	rec53 := server.NewServerWithFullConfig(cfg.DNS.Listen, cfg.Warmup, cfg.Snapshot, cfg.Hosts, cfg.Forwarding)
	// Restore cache from snapshot before starting listeners so the cache is
	// warm before the first DNS query arrives.  This is synchronous and completes
	// in < 5 ms on typical snapshot files.  Missing file → silent no-op; any
	// other error → warn and continue (degraded to cold-cache behaviour).
	if n, err := server.LoadSnapshot(cfg.Snapshot); err != nil {
		monitor.Rec53Log.Warnf("[SNAPSHOT] failed to load snapshot, starting with cold cache: %v", err)
	} else if n > 0 {
		monitor.Rec53Log.Infof("[SNAPSHOT] restored %d cache entries from %s", n, cfg.Snapshot.File)
	}

	monitor.Rec53Log.Debugf("starting DNS server")
	errChan := rec53.Run()

	monitor.Rec53Log.Infof("rec53 started, listening on %s, metrics on %s", cfg.DNS.Listen, cfg.DNS.Metric)

	// Wait for shutdown signal or server error
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	waitForSignal(sig, errChan)

	// Cancel pprof context first (triggers graceful pprof server shutdown)
	pprofCancel()

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gracefulShutdown(ctx, rec53.Shutdown, monitor.ShutdownMetric)

	monitor.Rec53Log.Info("rec53 stopped")
}
