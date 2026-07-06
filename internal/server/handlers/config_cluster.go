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

	if err = s.changeBackupConfig(r.Context(), func(config *dto.Config) ([]string, error) {
		if _, exists := config.AerospikeClusters[name]; exists {
			return nil, fmt.Errorf("add Aerospike cluster %q: %w", name, model.ErrAlreadyExists)
		}
		config.AerospikeClusters[name] = newCluster
		return nil, nil
	}); err != nil {
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
// @Failure     404 {string} string "The specified cluster was not found"
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

	if err = s.changeBackupConfig(r.Context(), func(config *dto.Config) ([]string, error) {
		if _, exists := config.AerospikeClusters[clusterName]; !exists {
			return nil, fmt.Errorf("update Aerospike cluster %q: %w", clusterName, model.ErrNotFound)
		}
		config.AerospikeClusters[clusterName] = updatedCluster
		return nil, nil
	}, withNamespaceValidation); err != nil {
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

	err := s.changeBackupConfig(r.Context(), func(config *dto.Config) ([]string, error) {
		if _, exists := config.AerospikeClusters[clusterName]; !exists {
			return nil, fmt.Errorf("delete Aerospike cluster %q: %w", clusterName, model.ErrNotFound)
		}
		if err := ensureClusterNotInUse(config, clusterName); err != nil {
			return nil, err
		}
		delete(config.AerospikeClusters, clusterName)
		return nil, nil
	})
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
