package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

func (s *Service) ConfigActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.readConfig(w)
	case http.MethodPut:
		s.updateConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// readConfig
// @Summary     Returns the configuration for the service.
// @ID	        readConfig
// @Tags        Configuration
// @Router      /v1/config [get]
// @Produce     json
// @Success     200 {object} dto.Config
// @Failure     500 {string} string
func (s *Service) readConfig(w http.ResponseWriter) {
	hLogger := s.logger.With(slog.String("handler", "readConfig"))

	configuration, err := dto.Serialize(dto.NewConfigFromModel(s.config), dto.JSON)
	if err != nil {
		hLogger.Error("failed to parse service configuration",
			slog.Any("error", err),
		)
		http.Error(w, "failed to parse service configuration", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err = w.Write(configuration); err != nil {
		hLogger.Error("failed to write response",
			slog.Any("error", err),
		)
	}
}

// updateConfig
// @Summary     Updates the configuration for the service.
// @ID 	        updateConfig
// @Tags        Configuration
// @Router      /v1/config [put]
// @Accept      json
// @Param       config body dto.Config true "Configuration details"
// @Success     200
// @Failure     400 {string} string
func (s *Service) updateConfig(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "updateConfig"))

	newConfig, err := dto.NewConfigFromReader(r.Body, dto.JSON)
	if err != nil {
		hLogger.Error("failed to decode new configuration",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newConfigModel, err := newConfig.ToModel()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		config.CopyFrom(newConfigModel)
		return nil
	})

	if err != nil {
		hLogger.Error("failed to update config",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// ApplyConfig  read and apply configuration from file.
// @Summary     Applies the configuration for the service.
// @ID          applyConfig
// @Tags        Configuration
// @Router      /v1/config/apply [post]
// @Accept      json
// @Success     200
// @Failure     400 {string} string
func (s *Service) ApplyConfig(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "ApplyConfig"))

	config, err := s.configurationManager.Read(r.Context())
	if err != nil {
		hLogger.Error("failed to read config",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = s.applyConfig(config)
	if err != nil {
		hLogger.Error("failed to apply config",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Service) changeConfig(ctx context.Context, updateFunc func(*model.Config) error) error {
	s.Lock()
	defer s.Unlock()

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

func (s *Service) applyConfig(c *model.Config) error {
	s.Lock()
	defer s.Unlock()

	s.config.CopyFrom(c)

	return s.configApplier.ApplyNewConfig()
}
