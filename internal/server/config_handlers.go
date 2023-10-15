package server

import (
	"encoding/json"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"net/http"
)

var ConfigurationManager service.ConfigurationManager

// readConfig
// @Summary Returns the configuration for the service.
// @Router /config [get]
// @Success 200 {array} model.Config
func (ws *HTTPServer) readConfig(w http.ResponseWriter) {
	configuration, err := json.MarshalIndent(ws.config, "", "    ") // pretty print
	if err != nil {
		http.Error(w, "Failed to parse service configuration", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(configuration)
}

// updateConfig
// @Summary Updates the configuration for the service.
// @Router /config [post]
// @Success 200 {array} model.Config
func (ws *HTTPServer) updateConfig(w http.ResponseWriter, r *http.Request) {
	var newConfig model.Config

	err := json.NewDecoder(r.Body).Decode(&newConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = ConfigurationManager.WriteConfiguration(&newConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}
