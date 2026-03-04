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
)

var (
	listenAddr  = flag.String("listen", "127.0.0.1:5353", "DNS server listen address (host:port)")
	metricAddr  = flag.String("metric", ":9999", "Prometheus metrics listen address (host:port)")
	logLevel    = flag.String("log-level", "info", "Log level: debug, info, warn, error")
	showVersion = flag.Bool("version", false, "Show version information")
)

// Version information (set via ldflags during build)
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("rec53 version: %s\nbuilt: %s\n", version, buildTime)
		return
	}

	// Initialize logger
	monitor.InitLogger()
	defer monitor.Rec53Log.Sync()
	monitor.SetLogLevel(parseLogLevel(*logLevel).Level())

	// Initialize metrics server
	monitor.InitMetricWithAddr(*metricAddr)

	// Start DNS server
	rec53 := server.NewServer(*listenAddr)
	errChan := rec53.Run()

	monitor.Rec53Log.Infof("rec53 started, listening on %s, metrics on %s", *listenAddr, *metricAddr)

	// Wait for shutdown signal or server error
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		if err != nil {
			monitor.Rec53Log.Errorf("Server error: %s", err.Error())
		}
	case s := <-sig:
		monitor.Rec53Log.Infof("Signal (%v) received, shutting down gracefully", s)
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown DNS server
	if err := rec53.Shutdown(ctx); err != nil {
		monitor.Rec53Log.Errorf("DNS server shutdown error: %s", err.Error())
	}

	// Shutdown metrics server
	if err := monitor.ShutdownMetric(ctx); err != nil {
		monitor.Rec53Log.Errorf("Metrics server shutdown error: %s", err.Error())
	}

	monitor.Rec53Log.Info("rec53 stopped")
}
