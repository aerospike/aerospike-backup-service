package handlers

import (
	"fmt"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
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
// @Failure     500 {string} string
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
	r.Body.Close()

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		cluster, err := newCluster.ToModel(config)
		if err != nil {
			return fmt.Errorf("invalid cluster %q: %w", name, err)
		}

		return config.AddCluster(name, cluster)
	})
	if err != nil {
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
// @Failure     500 {string} string
func (s *Service) ReadAerospikeClusters(w http.ResponseWriter, _ *http.Request) {
	backupConfig := s.config.BackupConfigCopy()
	clusters := dto.ConvertModelMapToDTO(
		backupConfig.AerospikeClusters,
		func(m *model.AerospikeCluster) *dto.AerospikeCluster {
			return dto.NewClusterFromModel(m, backupConfig)
		})

	httpOK(w, clusters)
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
// @Failure     404 {string} string "The specified cluster could not be found"
// @Failure     500 {string} string "The specified cluster could not be found"
func (s *Service) ReadAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	clusterName := r.PathValue("name")
	if clusterName == "" {
		httpError(w, errMissingClusterName)
		return
	}
	backupConfig := s.config.BackupConfigCopy()
	cluster, ok := backupConfig.AerospikeClusters[clusterName]
	if !ok {
		httpError(w, errNotFound("cluster", clusterName))
		return
	}

	httpOK(w, dto.NewClusterFromModel(cluster, backupConfig))
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
	clusterName := r.PathValue("name")
	if clusterName == "" {
		httpError(w, errMissingClusterName)
		return
	}

	updatedCluster, err := dto.NewClusterFromReader(r.Body, decoder.JSON)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}
	r.Body.Close()
	cluster, err := updatedCluster.ToModel(s.config)
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		err := config.UpdateCluster(clusterName, cluster)
		if err != nil {
			return err
		}

		// validate that new cluster has all required NSs (under the lock)
		s.nsValidator.ValidateConfig(config)
		return nil
	})

	if err != nil {
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
	clusterName := r.PathValue("name")
	if clusterName == "" {
		httpError(w, errMissingClusterName)
		return
	}

	err := s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.DeleteCluster(clusterName)
	})
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
