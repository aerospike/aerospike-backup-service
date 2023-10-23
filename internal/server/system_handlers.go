package server

import (
	"fmt"
	"net/http"

	"github.com/aerospike/backup/internal/util"
)

// @Summary     Root endpoint.
// @ID			root
// @Tags        System
// @Router      / [get]
// @Success 	200
func rootActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
	}
	fmt.Fprintf(w, "")
}

// @Summary     Health endpoint.
// @ID			health
// @Tags        System
// @Router      /health [get]
// @Success 	200
func healthActionHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "Ok")
}

// @Summary     Readiness endpoint.
// @ID			ready
// @Tags        System
// @Router      /ready [get]
// @Success 	200
func readyActionHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "Ok")
}

// @Summary     Returns application version.
// @ID			version
// @Tags        System
// @Router      /version [get]
// @Success 	200 {string} string "version"
func versionActionHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprint(w, util.Version)
}
