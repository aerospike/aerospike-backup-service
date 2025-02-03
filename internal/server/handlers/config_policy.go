package handlers

import (
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

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
	newPolicy, err := dto.NewBackupPolicyFromReader(r.Body, dto.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}
	r.Body.Close()
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingPolicyName)
		return
	}
	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.AddPolicy(name, newPolicy.ToModel())
	})
	if err != nil {
		httpError(w, errBadRequest(err))
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
	policies := dto.ConvertModelMapToDTO(s.config.BackupConfigCopy().BackupPolicies, dto.NewBackupPolicyFromModel)
	httpOK(w, policies)
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
	updatedPolicy, err := dto.NewBackupPolicyFromReader(r.Body, dto.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}
	r.Body.Close()
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingPolicyName)
		return
	}

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.UpdatePolicy(name, updatedPolicy.ToModel())
	})
	if err != nil {
		httpError(w, errBadRequest(err))
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
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingPolicyName)
		return
	}

	err := s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.DeletePolicy(name)
	})
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
