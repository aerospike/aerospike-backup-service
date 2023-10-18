package server

import (
	"fmt"
	"net/http"

	"github.com/aerospike/backup/internal/util"
)

// @Summary     Root endpoint.
// @Tags        System
// @Router      / [get]
// @Success 	200 ""
func rootActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
	}
	fmt.Fprintf(w, "")
}

// @Summary     Health endpoint.
// @Tags        System
// @Router      /health [get]
// @Success 	200  "Ok"
func healthActionHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "Ok")
}

// @Summary     Readiness endpoint.
// @Tags        System
// @Router      /ready [get]
// @Success 	200  "Ok"
func readyActionHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "Ok")
}

// @Summary     Returns application version.
// @Tags        System
// @Router      /version [get]
// @Success 	200 {string} string "version"
func versionActionHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprint(w, util.Version)
}
