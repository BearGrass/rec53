package monitor

import (
	"context"
	"net/http"
	"net/http/pprof"
	"time"
)

// StartPprofServer starts a pprof HTTP server on the given address.
// The server shuts down gracefully when ctx is cancelled.
// Errors during startup are logged but do not affect the caller.
func StartPprofServer(ctx context.Context, listenAddr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			Rec53Log.Errorf("[PPROF] server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			Rec53Log.Errorf("[PPROF] shutdown error: %v", err)
		}
	}()
}
