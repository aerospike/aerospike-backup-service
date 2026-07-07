package handlers

import (
	"fmt"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// AddPolicy
// @Summary     Adds a policy to the config.
// @ID          addPolicy
// @Tags        Configuration
// @Router      /v1/config/policies/{name} [post]
// @Accept      json
// @Param       name path string true "Backup policy name"
// @Param       policy body dto.BackupPolicy true "Backup policy details"
// @Success     201
// @Failure     400 {string} string
func (s *Service) AddPolicy(w http.ResponseWriter, r *http.Request) {
	newPolicy, err := dto.NewBackupPolicyFromReader(r.Body, decoder.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}

	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingPolicyName)
		return
	}

	if err = s.changeBackupConfig(r.Context(), func(config *dto.Config) ([]string, error) {
		if _, exists := config.BackupPolicies[name]; exists {
			return nil, fmt.Errorf("add backup policy %q: %w", name, model.ErrAlreadyExists)
		}
		config.BackupPolicies[name] = newPolicy
		return nil, nil
	}); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// ReadPolicies reads all backup policies from the configuration.
// @Summary     Reads all policies from the configuration.
// @ID	        readPolicies
// @Tags        Configuration
// @Router      /v1/config/policies [get]
// @Produce     json
// @Success  	200 {object} map[string]dto.BackupPolicy
func (s *Service) ReadPolicies(w http.ResponseWriter, _ *http.Request) {
	policies := dto.ConvertModelMapToDTO(s.config.BackupConfigCopy().BackupPolicies, dto.NewBackupPolicyFromModel)
	httpOK(w, policies)
}

// ReadPolicy reads a specific backup policy from the configuration given its name.
// @Summary     Reads a backup policy from the configuration given its name.
// @ID	        readPolicy
// @Tags        Configuration
// @Router      /v1/config/policies/{name} [get]
// @Param       name path string true "Backup policy name"
// @Produce     json
// @Success  	200 {object} dto.BackupPolicy
// @Response    400 {string} string
// @Failure     404 {string} string "The specified policy was not found"
func (s *Service) ReadPolicy(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingPolicyName)
		return
	}
	policy, ok := s.config.BackupConfigCopy().BackupPolicies[name]
	if !ok {
		httpError(w, errNotFound("policy", name))
		return
	}

	httpOK(w, dto.NewBackupPolicyFromModel(policy))
}

// UpdatePolicy updates an existing policy in the configuration.
// @Summary     Updates an existing policy in the configuration.
// @ID 	        updatePolicy
// @Tags        Configuration
// @Router      /v1/config/policies/{name} [put]
// @Accept      json
// @Param       name path string true "Backup policy name"
// @Param       policy body dto.BackupPolicy true "Backup policy details"
// @Success     200
// @Failure     400 {string} string
func (s *Service) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	updatedPolicy, err := dto.NewBackupPolicyFromReader(r.Body, decoder.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}

	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingPolicyName)
		return
	}

	if err = s.changeBackupConfig(r.Context(), func(config *dto.Config) ([]string, error) {
		if _, exists := config.BackupPolicies[name]; !exists {
			return nil, fmt.Errorf("update backup policy %q: %w", name, model.ErrNotFound)
		}
		config.BackupPolicies[name] = updatedPolicy
		return nil, nil
	}); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeletePolicy
// @Summary     Deletes a policy from the configuration by name.
// @ID          deletePolicy
// @Tags        Configuration
// @Router      /v1/config/policies/{name} [delete]
// @Param       name path string true "Backup policy name"
// @Success     204
// @Failure     400 {string} string
func (s *Service) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingPolicyName)
		return
	}

	err := s.changeBackupConfig(r.Context(), func(config *dto.Config) ([]string, error) {
		if _, exists := config.BackupPolicies[name]; !exists {
			return nil, fmt.Errorf("delete backup policy %q: %w", name, model.ErrNotFound)
		}
		if err := ensurePolicyNotInUse(config, name); err != nil {
			return nil, err
		}
		delete(config.BackupPolicies, name)
		return nil, nil
	})
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
