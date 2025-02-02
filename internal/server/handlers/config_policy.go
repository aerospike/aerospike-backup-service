package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

const policyNameNotSpecifiedMsg = "Policy name is not specified"

func (s *Service) ConfigPolicyActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.AddPolicy(w, r)
	case http.MethodGet:
		s.ReadPolicy(w, r)
	case http.MethodPut:
		s.UpdatePolicy(w, r)
	case http.MethodDelete:
		s.DeletePolicy(w, r)
	}
}

// AddPolicy
// @Summary     Adds a policy to the config.
// @ID          AddPolicy
// @Tags        Configuration
// @Router      /v1/config/policies/{name} [post]
// @Accept      json
// @Param       name path string true "Backup policy name"
// @Param       policy body dto.BackupPolicy true "Backup policy details"
// @Success     201
// @Failure     400 {string} string
//
//nolint:dupl
func (s *Service) AddPolicy(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "addPolicy"))

	newPolicy, err := dto.NewBackupPolicyFromReader(r.Body, dto.JSON)
	if err != nil {
		hLogger.Error("failed to decode policy",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()
	name := r.PathValue("name")
	if name == "" {
		hLogger.Error("policy name required")
		http.Error(w, policyNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.AddPolicy(name, newPolicy.ToModel())
	})
	if err != nil {
		hLogger.Error("failed to add policy",
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

	policies := dto.ConvertModelMapToDTO(s.config.BackupConfigCopy().BackupPolicies, dto.NewBackupPolicyFromModel)
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

// ReadPolicy reads a specific backup policy from the configuration given its name.
// @Summary     Reads a backup policy from the configuration given its name.
// @ID	        ReadPolicy
// @Tags        Configuration
// @Router      /v1/config/policies/{name} [get]
// @Param       name path string true "Backup policy name"
// @Produce     json
// @Success  	200 {object} dto.BackupPolicy
// @Response    400 {string} string
// @Failure     404 {string} string "The specified policy could not be found"
// @Failure     500 {string} string "The specified policy could not be found"
func (s *Service) ReadPolicy(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "readPolicy"))

	policyName := r.PathValue("name")
	if policyName == "" {
		hLogger.Error("policy name required")
		http.Error(w, policyNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	policy, ok := s.config.BackupConfigCopy().BackupPolicies[policyName]
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

// UpdatePolicy updates an existing policy in the configuration.
// @Summary     Updates an existing policy in the configuration.
// @ID 	        UpdatePolicy
// @Tags        Configuration
// @Router      /v1/config/policies/{name} [put]
// @Accept      json
// @Param       name path string true "Backup policy name"
// @Param       policy body dto.BackupPolicy true "Backup policy details"
// @Success     200
// @Failure     400 {string} string
//
//nolint:dupl
func (s *Service) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "updatePolicy"))

	updatedPolicy, err := dto.NewBackupPolicyFromReader(r.Body, dto.JSON)
	if err != nil {
		hLogger.Error("failed to decode policy",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()
	name := r.PathValue("name")
	if name == "" {
		hLogger.Error("policy name required")
		http.Error(w, policyNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.UpdatePolicy(name, updatedPolicy.ToModel())
	})
	if err != nil {
		hLogger.Error("failed to update policy",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeletePolicy
// @Summary     Deletes a policy from the configuration by name.
// @ID          DeletePolicy
// @Tags        Configuration
// @Router      /v1/config/policies/{name} [delete]
// @Param       name path string true "Backup policy name"
// @Success     204
// @Failure     400 {string} string
func (s *Service) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "deletePolicy"))

	policyName := r.PathValue("name")
	if policyName == "" {
		hLogger.Error("policy name required")
		http.Error(w, policyNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}

	err := s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.DeletePolicy(policyName)
	})
	if err != nil {
		hLogger.Error("failed to delete policy",
			slog.String("name", policyName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
