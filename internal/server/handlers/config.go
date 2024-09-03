package handlers

import (
	"log/slog"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v2/internal/server/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service"
)

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
		// We won't log config as it is not secure.
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
		// We won't log config as it is not secure.
		hLogger.Error("failed to decode new configuration",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.config, err = newConfig.ToModel()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = s.configurationManager.WriteConfiguration(s.config)
	if err != nil {
		// We won't log config as it is not secure.
		hLogger.Error("failed to update configuration",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ApplyConfig
// @Summary     Applies the configuration for the service.
// @ID          applyConfig
// @Tags        Configuration
// @Router      /v1/config/apply [post]
// @Accept      json
// @Success     200
// @Failure     400 {string} string
func (s *Service) ApplyConfig(w http.ResponseWriter, _ *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "ApplyConfig"))

	handlers, err := service.ApplyNewConfig(s.scheduler, s.config, s.backupBackends, s.clientManger)
	if err != nil {
		hLogger.Error("failed to apply new config",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.handlerHolder = handlers
	w.WriteHeader(http.StatusOK)
}

func (s *Service) ConfigActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.readConfig(w)
	case http.MethodPut:
		s.updateConfig(w, r)
	}
}
