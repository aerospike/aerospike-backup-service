package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v2/internal/server/dto"
	"github.com/gorilla/mux"
)

const policyNameNotSpecifiedMsg = "Policy name is not specified"

func (s *Service) ConfigPolicyActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.addPolicy(w, r)
	case http.MethodGet:
		s.readPolicy(w, r)
	case http.MethodPut:
		s.updatePolicy(w, r)
	case http.MethodDelete:
		s.deletePolicy(w, r)
	}
}

// addPolicy
// @Summary     Adds a policy to the config.
// @ID          addPolicy
// @Tags        Configuration
// @Router      /v1/config/policies/{name} [post]
// @Accept      json
// @Param       name path string true "Backup policy name"
// @Param       policy body dto.BackupPolicy true "Backup policy details"
// @Success     201
// @Failure     400 {string} string
//
//nolint:dupl
func (s *Service) addPolicy(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "addPolicy"))

	var request dto.BackupPolicy
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		hLogger.Error("failed to decode request body",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()
	name := mux.Vars(r)["name"]
	if name == "" {
		hLogger.Error("policy name required")
		http.Error(w, policyNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	err = s.config.AddPolicy(name, request.ToModel())
	if err != nil {
		hLogger.Error("failed to add policy",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = s.configurationManager.WriteConfiguration(s.config)
	if err != nil {
		hLogger.Error("failed to write configuration",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// ReadPolicies reads all backup policies from the configuration.
// @Summary     Reads all policies from the configuration.
// @ID	        ReadPolicies
// @Tags        Configuration
// @Router      /v1/config/policies [get]
// @Produce     json
// @Success  	200 {object} map[string]dto.BackupPolicy
// @Failure     500 {string} string
func (s *Service) ReadPolicies(w http.ResponseWriter, _ *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "ReadPolicies"))

	policies := dto.ConvertModelMapToDTO(s.config.BackupPolicies, dto.NewBackupPolicyFromModel)
	jsonResponse, err := dto.Serialize(policies, dto.JSON)
	if err != nil {
		hLogger.Error("failed to marshal backup policies",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		hLogger.Error("failed to write response",
			slog.String("response", string(jsonResponse)),
			slog.Any("error", err),
		)
	}
}

// readPolicy reads a specific backup policy from the configuration given its name.
// @Summary     Reads a backup policy from the configuration given its name.
// @ID	        readPolicy
// @Tags        Configuration
// @Router      /v1/config/policies/{name} [get]
// @Param       name path string true "Backup policy name"
// @Produce     json
// @Success  	200 {object} dto.BackupPolicy
// @Response    400 {string} string
// @Failure     404 {string} string "The specified policy could not be found"
// @Failure     500 {string} string "The specified policy could not be found"
func (s *Service) readPolicy(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "readPolicy"))

	policyName := mux.Vars(r)["name"]
	if policyName == "" {
		hLogger.Error("policy name required")
		http.Error(w, policyNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	policy, ok := s.config.BackupPolicies[policyName]
	if !ok {
		hLogger.Error("policy not found")
		http.Error(w, fmt.Sprintf("policy %s could not be found", policyName), http.StatusNotFound)
		return
	}
	jsonResponse, err := dto.Serialize(dto.NewBackupPolicyFromModel(policy), dto.JSON)
	if err != nil {
		hLogger.Error("failed to marshal policy",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		hLogger.Error("failed to write response",
			slog.String("response", string(jsonResponse)),
			slog.Any("error", err),
		)
	}
}

// updatePolicy updates an existing policy in the configuration.
// @Summary     Updates an existing policy in the configuration.
// @ID 	        updatePolicy
// @Tags        Configuration
// @Router      /v1/config/policies/{name} [put]
// @Accept      json
// @Param       name path string true "Backup policy name"
// @Param       policy body dto.BackupPolicy true "Backup policy details"
// @Success     200
// @Failure     400 {string} string
//
//nolint:dupl
func (s *Service) updatePolicy(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "updatePolicy"))

	var request dto.BackupPolicy
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		hLogger.Error("failed to decode request body",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()
	name := mux.Vars(r)["name"]
	if name == "" {
		hLogger.Error("policy name required")
		http.Error(w, policyNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	err = s.config.UpdatePolicy(name, request.ToModel())
	if err != nil {
		hLogger.Error("failed to update policy",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = s.configurationManager.WriteConfiguration(s.config)
	if err != nil {
		hLogger.Error("failed to write configuration",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// deletePolicy
// @Summary     Deletes a policy from the configuration by name.
// @ID          deletePolicy
// @Tags        Configuration
// @Router      /v1/config/policies/{name} [delete]
// @Param       name path string true "Backup policy name"
// @Success     204
// @Failure     400 {string} string
//
//nolint:dupl // Each handler must be in separate func. No duplication.
func (s *Service) deletePolicy(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "deletePolicy"))

	policyName := mux.Vars(r)["name"]
	if policyName == "" {
		hLogger.Error("policy name required")
		http.Error(w, policyNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	err := s.config.DeletePolicy(policyName)
	if err != nil {
		hLogger.Error("failed to delete policy",
			slog.String("name", policyName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = s.configurationManager.WriteConfiguration(s.config)
	if err != nil {
		hLogger.Error("failed to write configuration",
			slog.String("name", policyName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
