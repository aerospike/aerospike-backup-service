package handlers

import (
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
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
func (s *Service) ReadConfig(w http.ResponseWriter, r *http.Request) {
	httpOK(w, s.configManager.ReadConfig(r.Context()))
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

	err = s.configManager.UpdateConfig(r.Context(), newConfig)
	if err != nil {
		httpError(w, errBadRequest(err))
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
	err := s.configManager.ApplyConfig(r.Context())
	if err != nil {
		httpError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
