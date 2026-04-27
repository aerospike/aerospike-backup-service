//go:build debug

package main

import (
	"errors"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"time"
)

func init() {
	go func() {
		// Binds to loopback only. In containers, access via: docker exec <id> curl localhost:6060/debug/pprof/
		addr := "localhost:6060"
		slog.Info("Starting profiler", "addr", addr)
		srv := &http.Server{
			Addr:              addr,
			ReadHeaderTimeout: 5 * time.Second,
		}
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Profiler stopped", "error", err)
		}
	}()
}
