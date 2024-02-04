package server

import (
	"fmt"
	"log/slog"
	"net/http"

	_ "github.com/aerospike/backup/docs" // auto-generated Swagger spec
	"github.com/aerospike/backup/internal/util"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger/v2"
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
		slog.Error("failed to write response", "err", err)
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
		slog.Error("failed to write response", "err", err)
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
		slog.Error("failed to write response", "err", err)
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
		slog.Error("failed to write response", "err", err)
	}
}

// @Summary     Prometheus metrics endpoint.
// @ID          metrics
// @Tags        System
// @Router      /metrics [get]
// @Success 	200
func metricsActionHandler() http.Handler {
	return promhttp.Handler()
}

// @Summary     OpenAPI specification endpoint.
// @ID          api-docs
// @Tags        System
// @Router      /api-docs/ [get]
// @Success 	200
func apiDocsActionHandler() http.Handler {
	return httpSwagger.Handler()
}
