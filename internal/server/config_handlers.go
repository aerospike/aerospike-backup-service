//nolint:dupl
package server

import (
	"encoding/json"
	"net/http"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
)

var ConfigurationManager service.ConfigurationManager

// readConfig
// @Summary     Returns the configuration for the service.
// @ID	        readConfig
// @Tags        Configuration
// @Router      /v1/config [get]
// @Produce     json
// @Success     200 {object} model.Config
// @Failure     400 {string} string
func (ws *HTTPServer) readConfig(w http.ResponseWriter) {
	configuration, err := json.MarshalIndent(ws.config, "", "    ") // pretty print
	if err != nil {
		http.Error(w, "failed to parse service configuration", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(configuration)
}

// updateConfig
// @Summary     Updates the configuration for the service.
// @ID 	        updateConfig
// @Tags        Configuration
// @Router      /v1/config [put]
// @Accept      json
// @Param       config body model.Config true "Configuration details"
// @Success     200
// @Failure     400 {string} string
func (ws *HTTPServer) updateConfig(w http.ResponseWriter, r *http.Request) {
	var newConfig model.Config

	err := json.NewDecoder(r.Body).Decode(&newConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err = newConfig.Validate(); err != nil {
		http.Error(w, "invalid configuration: "+err.Error(), http.StatusBadRequest)
		return
	}
	ws.config = &newConfig
	err = ConfigurationManager.WriteConfiguration(&newConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// applyConfig
// @Summary     Applies the configuration for the service.
// @ID          applyConfig
// @Tags        Configuration
// @Router      /v1/config/apply [post]
// @Accept      json
// @Success     200
// @Failure     400 {string} string
func (ws *HTTPServer) applyConfig(w http.ResponseWriter, _ *http.Request) {
	err := service.ApplyNewConfig(ws.scheduler, ws.config, ws.backupBackends)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}
