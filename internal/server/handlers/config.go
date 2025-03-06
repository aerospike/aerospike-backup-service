package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/validation"
)

func (s *Service) ConfigActionHandler(_ http.ResponseWriter, _ *http.Request) {

}

// ReadConfig
// @Summary     Returns the configuration for the service.
// @ID	        readConfig
// @Tags        Configuration
// @Router      /v1/config [get]
// @Produce     json
// @Success     200 {object} dto.Config
// @Failure     500 {string} string
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
func (s *Service) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	newConfig, err := dto.NewConfigFromReader(r.Body, dto.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}

	// validate static fields.
	oldConfig := dto.NewConfigFromModel(s.config)
	if err := validation.ValidateStaticFieldChanges(oldConfig, newConfig); err != nil {
		httpError(w, errBadRequest(fmt.Errorf("static configuration has changed: %w", err)))
		return
	}

	newConfigModel, err := newConfig.ToModel(s.nsValidator)
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		config.SetBackupConfig(newConfigModel.BackupConfigCopy())
		return nil
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
	config, err := s.configurationManager.Read(r.Context())
	if err != nil {
		httpError(w, fmt.Errorf("failed to read configuration: %w", err))
		return
	}

	// validate static fields.
	newConfig := dto.NewConfigFromModel(s.config)
	oldConfig := dto.NewConfigFromModel(config)
	if err := validation.ValidateStaticFieldChanges(oldConfig, newConfig); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	backupConfig := config.BackupConfigCopy()
	s.config.SetBackupConfig(backupConfig)
	slog.Info("Apply new configuration")
	err = s.configApplier.ApplyNewRoutines(backupConfig.BackupRoutines)

	if err != nil {
		httpError(w, err)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Service) changeConfig(ctx context.Context, updateFunc func(*model.Config) error) error {
	err := updateFunc(s.config)
	if err != nil {
		return fmt.Errorf("cannot update configuration: %w", err)
	}

	err = s.configurationManager.Write(ctx, s.config)
	if err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	slog.Info("change configuration")
	err = s.configApplier.ApplyNewRoutines(s.config.Routines())
	if err != nil {
		return fmt.Errorf("failed to apply new configuration: %w", err)
	}

	return nil
}
