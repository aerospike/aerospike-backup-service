package server

import (
	"encoding/json"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"log/slog"
	"net/http"
)

// addPolicy
// @Summary     Adds a policy to the config.
// @ID          addPolicy
// @Tags        Configuration
// @Router      /config/policy [post]
// @Accept      json
// @Param       name query string true "policy name"
// @Param       storage body model.BackupPolicy true "backup policy"
// @Success     201
// @Failure     400 {string} string
func (ws *HTTPServer) addPolicy(w http.ResponseWriter, r *http.Request) {
	var newPolicy model.BackupPolicy
	err := json.NewDecoder(r.Body).Decode(&newPolicy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "policy name is required", http.StatusBadRequest)
		return
	}
	err = service.AddPolicy(ws.config, name, &newPolicy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = ConfigurationManager.WriteConfiguration(ws.config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// readPolicies reads all backup policies from the configuration.
// @Summary     Reads all policies from the configuration.
// @ID	        readPolicies
// @Tags        Configuration
// @Router      /config/policy [get]
// @Produce     json
// @Success  	200 {object} map[string]model.BackupPolicy
// @Failure     400 {string} string
func (ws *HTTPServer) readPolicies(w http.ResponseWriter) {
	jsonResponse, err := json.Marshal(ws.config.BackupPolicies)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		slog.Error("failed to write response", "err", err)
	}
}

// updatePolicy updates an existing policy in the configuration.
// @Summary     Updates an existing policy in the configuration.
// @ID 	        updatePolicy
// @Tags        Configuration
// @Router      /config/policy [put]
// @Accept      json
// @Param       name query string true "policy name"
// @Param       storage body model.BackupPolicy true "backup policy"
// @Success     200
// @Failure     400 {string} string
func (ws *HTTPServer) updatePolicy(w http.ResponseWriter, r *http.Request) {
	var updatedPolicy model.BackupPolicy
	err := json.NewDecoder(r.Body).Decode(&updatedPolicy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "policy name is required", http.StatusBadRequest)
		return
	}
	err = service.UpdatePolicy(ws.config, name, &updatedPolicy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = ConfigurationManager.WriteConfiguration(ws.config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// deletePolicy
// @Summary     Deletes a policy from the configuration by name.
// @ID          deletePolicy
// @Tags        Configuration
// @Router      /config/policy [delete]
// @Param       name query string true "Policy Name"
// @Success     204
// @Failure     400 {string} string
func (ws *HTTPServer) deletePolicy(w http.ResponseWriter, r *http.Request) {
	policyName := r.URL.Query().Get("name")
	if policyName == "" {
		http.Error(w, "Policy name is required", http.StatusBadRequest)
		return
	}

	err := service.DeletePolicy(ws.config, policyName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = ConfigurationManager.WriteConfiguration(ws.config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
