package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	backup "github.com/aerospike/aerospike-backup-service/v2"
	_ "github.com/aerospike/aerospike-backup-service/v2/docs" // auto-generated Swagger spec
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// RootActionHandler
// @Summary     Root endpoint.
// @ID	        root
// @Tags        System
// @Router      / [get]
// @Success 	200
func RootActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
	}
	_, err := fmt.Fprintf(w, "")
	if err != nil {
		slog.Error("failed to write response", "err", err)
	}
}

// HealthActionHandler
// @Summary     Health endpoint.
// @ID	        health
// @Tags        System
// @Router      /health [get]
// @Success 	200
func HealthActionHandler(w http.ResponseWriter, _ *http.Request) {
	_, err := fmt.Fprintf(w, "Ok")
	if err != nil {
		slog.Error("failed to write response", "err", err)
	}
}

// ReadyActionHandler
// @Summary     Readiness endpoint.
// @ID	        ready
// @Tags        System
// @Router      /ready [get]
// @Success 	200
func ReadyActionHandler(w http.ResponseWriter, _ *http.Request) {
	_, err := fmt.Fprintf(w, "Ok")
	if err != nil {
		slog.Error("failed to write response", "err", err)
	}
}

// VersionActionHandler
// @Summary     Returns application version.
// @ID	        version
// @Tags        System
// @Router      /version [get]
// @Success 	200 {string} string "version"
func VersionActionHandler(w http.ResponseWriter, _ *http.Request) {
	_, err := fmt.Fprint(w, backup.Version)
	if err != nil {
		slog.Error("failed to write response", "err", err)
	}
}

// MetricsActionHandler
// @Summary     Prometheus metrics endpoint.
// @ID          metrics
// @Tags        System
// @Router      /metrics [get]
// @Success 	200
func MetricsActionHandler() http.Handler {
	return promhttp.Handler()
}

// APIDocsActionHandler
// @Summary     OpenAPI specification endpoint.
// @Description Serves the API documentation in Swagger UI format.
// @ID          api-docs
// @Tags        System
// @Router      /api-docs/ [get]
// @Produce     html
// @Success 	200 {string} string
func APIDocsActionHandler() http.Handler {
	return httpSwagger.Handler()
}
