package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
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
	newConfig, err := dto.NewConfigFromReader(r.Body, decoder.JSON)
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

	newConfigModel, err := newConfig.ToModel()
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		config.SetBackupConfig(newConfigModel.BackupConfigCopy())
		s.nsValidator.Validate(r.Context(), config) // validate under the lock
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
	// ApplyConfig and changeConfig must be synchronized to prevent race conditions
	// where one operation reads/writes config while another is in the middle of updating it
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()

	config, err := s.configurationManager.Read(r.Context())
	if err != nil {
		httpError(w, fmt.Errorf("failed to read configuration: %w", err))
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
	err = s.configApplier.ApplyNewConfig()

	if err != nil {
		httpError(w, err)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Service) changeConfig(ctx context.Context, updateFunc func(*model.Config) error) error {
	// ApplyConfig and changeConfig must be synchronized to prevent race conditions
	// where one operation reads/writes config while another is in the middle of updating it
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()

	err := updateFunc(s.config)
	if err != nil {
		return fmt.Errorf("cannot update configuration: %w", err)
	}

	err = s.configurationManager.Write(ctx, s.config)
	if err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	err = s.configApplier.ApplyNewConfig()
	if err != nil {
		return fmt.Errorf("failed to apply new configuration: %w", err)
	}

	return nil
}
