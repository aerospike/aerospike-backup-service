package handlers

import (
	"fmt"
	"net/http"
	"strings"

	backup "github.com/aerospike/aerospike-backup-service/v3"
	_ "github.com/aerospike/aerospike-backup-service/v3/docs" // auto-generated Swagger spec
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
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
	_, _ = fmt.Fprintf(w, "")
}

// HealthActionHandler
// @Summary     Health endpoint.
// @ID	        health
// @Tags        System
// @Router      /health [get]
// @Success 	200
func HealthActionHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = fmt.Fprintf(w, "Ok")
}

// ReadyActionHandler
// @Summary     Readiness endpoint.
// @ID	        ready
// @Tags        System
// @Router      /ready [get]
// @Success 	200
func ReadyActionHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = fmt.Fprintf(w, "Ok")
}

// VersionActionHandler
// @Summary     Returns application version.
// @ID	        version
// @Tags        System
// @Router      /version [get]
// @Success 	200 {object} dto.VersionResponse
func VersionActionHandler(w http.ResponseWriter, _ *http.Request) {
	response := dto.VersionResponse{
		Version:   backup.Version,
		Commit:    backup.CommitHash,
		BuildTime: backup.BuildTime,
	}

	w.Header().Set("Content-Type", "application/json")

	body, _ := decoder.Marshal(response, decoder.JSON, false)
	_, _ = w.Write(body)
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler := httpSwagger.Handler()
		if strings.HasSuffix(r.URL.Path, "/api-docs/") {
			// When the user requests "/api-docs/", we need to serve "index.html".
			// We cannot use http.Redirect because the reverse proxy may strip path components or block redirects.
			// The Swagger handler extracts the file path from `RequestURI` using regex,
			// so we must rewrite `RequestURI` directly to point to "/api-docs/index.html".
			newReq := r.Clone(r.Context())
			newReq.RequestURI = strings.TrimSuffix(r.URL.Path, "/") + "/index.html"

			handler.ServeHTTP(w, newReq)
			return
		}

		// Normal path handling
		handler.ServeHTTP(w, r)
	})
}
