//go:build debug

package main

import (
	"log/slog"
	"net/http"
	_ "net/http/pprof"
)

func init() {
	go func() {
		// Binds to loopback only. In containers, access via: docker exec <id> curl localhost:6060/debug/pprof/
		addr := "localhost:6060"
		slog.Info("Starting profiler", "addr", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			slog.Error("Profiler stopped", "error", err)
		}
	}()
}
