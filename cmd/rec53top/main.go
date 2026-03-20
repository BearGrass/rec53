package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rec53/tui"
)

func main() {
	target := flag.String("target", tui.DefaultTarget, "Prometheus metrics endpoint to scrape")
	refresh := flag.Duration("refresh", 2*time.Second, "Dashboard refresh interval")
	timeout := flag.Duration("timeout", 1500*time.Millisecond, "Metrics scrape timeout")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := tui.Config{
		Target:          *target,
		RefreshInterval: *refresh,
		Timeout:         *timeout,
	}
	if err := tui.Run(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "rec53top: %v\n", err)
		os.Exit(1)
	}
}
