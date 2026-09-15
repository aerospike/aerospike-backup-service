package handlers

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/validation"
)

// ReadConfig
// @Summary     Returns the configuration for the service.
// @ID	        readConfig
// @Tags        Configuration
// @Router      /v1/config [get]
// @Produce     json
// @Success     200 {object} dto.Config
func (s *Service) ReadConfig(w http.ResponseWriter, _ *http.Request) {
	httpOK(w, dto.NewConfigFromModel(s.config))
}

// UpdateConfig
// @Summary     Updates the configuration for the service.
// @ID 	        updateConfig
// @Tags        Configuration
// @Router      /v1/config [put]
// @Accept      json
// @Param       config body dto.Config true "Configuration details"
// @Success     200
// @Failure     400 {string} string
// @Failure     503 {string} string "The configuration could not be persisted"
func (s *Service) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	newConfig, ok := decodeBody[dto.Config](w, r)
	if !ok {
		return
	}

	oldConfig := dto.NewConfigFromModel(s.config)

	// GET responses redact secrets as "[secret]". Before comparing or persisting a PUT, copy real secret
	// values from the stored config into the incoming payload wherever the sentinel appears,
	// so a GET-edit-PUT round trip does not overwrite secrets with the literal "[secret]".
	if err := decoder.MergeSecrets(newConfig, oldConfig); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	// validate static fields (after the merge, so a redacted secret does not count as a change).
	if err := validation.ValidateStaticFieldChanges(oldConfig, newConfig); err != nil {
		httpError(w, errBadRequest(fmt.Errorf("static configuration has changed: %w", err)))
		return
	}

	if err := newConfig.Validate(); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	newConfigModel, err := newConfig.ToModel()
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}
	if err := s.tlsProber.Probe(r.Context(), newConfigModel); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	if err := s.replaceConfig(r.Context(), newConfigModel); err != nil {
		httpError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// ApplyConfig  read and apply configuration from file.
// @Summary     Reloads the configuration from the config file.
// @ID          applyConfig
// @Tags        Configuration
// @Router      /v1/config/apply [post]
// @Accept      json
// @Success     200
// @Failure     400 {string} string
func (s *Service) ApplyConfig(w http.ResponseWriter, r *http.Request) {
	// ApplyConfig and changeConfig must be synchronized to prevent race conditions
	// where one operation reads/writes config while another is in the middle of updating it
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()

	config, err := s.configurationManager.Read(r.Context())
	if err != nil {
		httpError(w, fmt.Errorf("failed to read configuration: %w", err))
		return
	}
	if err := s.tlsProber.Probe(r.Context(), config); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	// validate static fields.
	newConfig := dto.NewConfigFromModel(s.config)
	oldConfig := dto.NewConfigFromModel(config)
	if err := validation.ValidateStaticFieldChanges(oldConfig, newConfig); err != nil {
		httpError(w, errBadRequest(fmt.Errorf("static configuration has changed: %w", err)))
		return
	}

	s.config.SetBackupConfig(config.BackupConfigCopy())
	s.config.InvalidateAllRoutines()
	err = s.configApplier.ApplyNewConfig()

	if err != nil {
		httpError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// replaceConfig persists newConfig and makes it the live configuration, rescheduling every
// routine it describes. The caller has already validated the change, so everything here is
// the commit.
func (s *Service) replaceConfig(ctx context.Context, newConfig *model.Config) error {
	// ApplyConfig and replaceConfig must be synchronized to prevent race conditions
	// where one operation reads/writes config while another is in the middle of updating it
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()

	s.nsValidator.Validate(ctx, newConfig) // validate under the lock

	return s.commitConfig(ctx, newConfig, routineNames(newConfig))
}

// routineNames lists every routine the configuration describes.
func routineNames(config *model.Config) []string {
	return slices.Collect(maps.Keys(config.BackupConfigCopy().BackupRoutines))
}
