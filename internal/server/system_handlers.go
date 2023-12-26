package server

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aerospike/backup/internal/util"
)

// @Summary     Root endpoint.
// @ID	        root
// @Tags        System
// @Router      / [get]
// @Success 	200
func rootActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
	}
	_, err := fmt.Fprintf(w, "")
	if err != nil {
		slog.Error("failed to write response", err)
	}
}

// @Summary     Health endpoint.
// @ID	        health
// @Tags        System
// @Router      /health [get]
// @Success 	200
func healthActionHandler(w http.ResponseWriter, _ *http.Request) {
	_, err := fmt.Fprintf(w, "Ok")
	if err != nil {
		slog.Error("failed to write response", err)
	}
}

// @Summary     Readiness endpoint.
// @ID	        ready
// @Tags        System
// @Router      /ready [get]
// @Success 	200
func readyActionHandler(w http.ResponseWriter, _ *http.Request) {
	_, err := fmt.Fprintf(w, "Ok")
	if err != nil {
		slog.Error("failed to write response", err)
	}
}

// @Summary     Returns application version.
// @ID	        version
// @Tags        System
// @Router      /version [get]
// @Success 	200 {string} string "version"
func versionActionHandler(w http.ResponseWriter, _ *http.Request) {
	_, err := fmt.Fprint(w, util.Version)
	if err != nil {
		slog.Error("failed to write response", err)
	}
}
