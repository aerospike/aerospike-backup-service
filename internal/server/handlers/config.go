package handlers

import (
	"fmt"
	"maps"
	"net/http"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
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

	// The whole backup configuration is replaced, so every routine is rescheduled.
	err := s.changeBackupConfig(r.Context(), func(config *dto.Config) ([]string, error) {
		*config = *newConfig
		return slices.Collect(maps.Keys(newConfig.BackupRoutines)), nil
	})
	if err != nil {
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
	// ApplyConfig and changeBackupConfig must be synchronized to prevent race conditions
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
	err = s.configApplier.ApplyNewConfig(r.Context())

	if err != nil {
		httpError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
