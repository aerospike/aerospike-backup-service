package handlers

import (
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
)

// AddAerospikeCluster
// @Summary     Adds an Aerospike cluster to the config.
// @ID          addCluster
// @Tags        Configuration
// @Router      /v1/config/clusters/{name} [post]
// @Accept      json
// @Param       name path string true "Aerospike cluster name"
// @Param       cluster body dto.AerospikeCluster true "Aerospike cluster details"
// @Success     201
// @Failure     400 {string} string
func (s *Service) AddAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingClusterName)
		return
	}

	newCluster, err := dto.NewClusterFromReader(r.Body, decoder.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}

	if err = s.configManager.AddAerospikeCluster(r.Context(), name, newCluster); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// ReadAerospikeClusters reads all Aerospike clusters from the configuration.
// @Summary     Reads all Aerospike clusters from the configuration.
// @ID	        readAllClusters
// @Tags        Configuration
// @Router      /v1/config/clusters [get]
// @Produce     json
// @Success  	200 {object} map[string]dto.AerospikeCluster
func (s *Service) ReadAerospikeClusters(w http.ResponseWriter, r *http.Request) {
	httpOK(w, s.configManager.ReadAerospikeClusters(r.Context()))
}

// ReadAerospikeCluster reads a specific Aerospike cluster from the configuration given its name.
// @Summary     Reads a specific Aerospike cluster from the configuration given its name.
// @ID	        readCluster
// @Tags        Configuration
// @Router      /v1/config/clusters/{name} [get]
// @Param       name path string true "Aerospike cluster name"
// @Produce     json
// @Success  	200 {object} dto.AerospikeCluster
// @Failure     400 {string} string
// @Failure     404 {string} string "The specified cluster was not found"
func (s *Service) ReadAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingClusterName)
		return
	}

	cluster, err := s.configManager.ReadAerospikeCluster(r.Context(), name)
	if err != nil {
		httpError(w, errNotFound("cluster", name))
		return
	}

	httpOK(w, cluster)
}

// UpdateAerospikeCluster updates an existing Aerospike cluster in the configuration.
// @Summary     Updates an existing Aerospike cluster in the configuration.
// @ID	        updateCluster
// @Tags        Configuration
// @Router      /v1/config/clusters/{name} [put]
// @Accept      json
// @Param       name path string true "Aerospike cluster name"
// @Param       cluster body dto.AerospikeCluster true "Aerospike cluster details"
// @Success     200
// @Failure     400 {string} string
func (s *Service) UpdateAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingClusterName)
		return
	}

	updatedCluster, err := dto.NewClusterFromReader(r.Body, decoder.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}

	if err = s.configManager.UpdateAerospikeCluster(r.Context(), name, updatedCluster); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteAerospikeCluster
// @Summary     Deletes a cluster from the configuration by name.
// @ID          deleteCluster
// @Tags        Configuration
// @Router      /v1/config/clusters/{name} [delete]
// @Param       name path string true "Aerospike cluster name"
// @Success     204
// @Failure     400 {string} string
func (s *Service) DeleteAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		httpError(w, errMissingClusterName)
		return
	}

	err := s.configManager.DeleteAerospikeCluster(r.Context(), name)
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
