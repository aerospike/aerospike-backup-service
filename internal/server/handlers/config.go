package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
)

var ConfigurationManager service.ConfigurationManager

// readConfig
// @Summary     Returns the configuration for the service.
// @ID	        readConfig
// @Tags        Configuration
// @Router      /v1/config [get]
// @Produce     json
// @Success     200 {object} model.Config
// @Failure     400 {string} string
func (s *Service) readConfig(w http.ResponseWriter) {
	configuration, err := json.MarshalIndent(s.config, "", "    ") // pretty print
	if err != nil {
		http.Error(w, "failed to parse service configuration", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(configuration)
}

// updateConfig
// @Summary     Updates the configuration for the service.
// @ID 	        updateConfig
// @Tags        Configuration
// @Router      /v1/config [put]
// @Accept      json
// @Param       config body model.Config true "Configuration details"
// @Success     200
// @Failure     400 {string} string
func (s *Service) updateConfig(w http.ResponseWriter, r *http.Request) {
	var newConfig model.Config

	err := json.NewDecoder(r.Body).Decode(&newConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err = newConfig.Validate(); err != nil {
		http.Error(w, "invalid configuration: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.config = &newConfig
	err = ConfigurationManager.WriteConfiguration(&newConfig)
	if err != nil {
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
	handlers, err := service.ApplyNewConfig(s.scheduler, s.config, s.backupBackends)
	if err != nil {
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
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) ConfigClusterActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.addAerospikeCluster(w, r)
	case http.MethodGet:
		s.readAerospikeCluster(w, r)
	case http.MethodPut:
		s.updateAerospikeCluster(w, r)
	case http.MethodDelete:
		s.deleteAerospikeCluster(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) ConfigStorageActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.addStorage(w, r)
	case http.MethodGet:
		s.readStorage(w, r)
	case http.MethodPut:
		s.updateStorage(w, r)
	case http.MethodDelete:
		s.deleteStorage(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) ConfigPolicyActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.addPolicy(w, r)
	case http.MethodGet:
		s.readPolicy(w, r)
	case http.MethodPut:
		s.updatePolicy(w, r)
	case http.MethodDelete:
		s.deletePolicy(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) ConfigRoutineActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.addRoutine(w, r)
	case http.MethodGet:
		s.readRoutine(w, r)
	case http.MethodPut:
		s.updateRoutine(w, r)
	case http.MethodDelete:
		s.deleteRoutine(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
