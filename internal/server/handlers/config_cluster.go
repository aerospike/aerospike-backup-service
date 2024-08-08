package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
)

const clusterNameNotSpecifiedMsg = "Cluster name is not specified"

// addAerospikeCluster
// @Summary     Adds an Aerospike cluster to the config.
// @ID          addCluster
// @Tags        Configuration
// @Router      /v1/config/clusters/{name} [post]
// @Accept      json
// @Param       name path string true "Aerospike cluster name"
// @Param       cluster body model.AerospikeCluster true "Aerospike cluster details"
// @Success     201
// @Failure     400 {string} string
func (s *Service) addAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "addAerospikeCluster"))

	var newCluster model.AerospikeCluster
	err := json.NewDecoder(r.Body).Decode(&newCluster)
	if err != nil {
		hLogger.Error("failed to decide request body",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()

	name := r.PathValue("name")
	if name == "" {
		hLogger.Error(clusterNameNotSpecifiedMsg,
			slog.String("name", name),
		)
		http.Error(w, clusterNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	err = service.AddCluster(s.config, name, &newCluster)
	if err != nil {
		hLogger.Error("failed to add cluster",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = s.configurationManager.WriteConfiguration(s.config)
	if err != nil {
		hLogger.Error("failed to write configuration",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
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
// @Success  	200 {object} map[string]model.AerospikeCluster
// @Failure     400 {string} string
func (s *Service) ReadAerospikeClusters(w http.ResponseWriter, _ *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "ReadAerospikeClusters"))

	clusters := s.config.AerospikeClusters
	jsonResponse, err := json.Marshal(clusters)
	if err != nil {
		hLogger.Error("failed to marshal clusters",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		hLogger.Error("failed to write response",
			slog.String("response", string(jsonResponse)),
			slog.Any("error", err),
		)
		slog.Error("failed to write response", "err", err)
	}
}

// readAerospikeCluster reads a specific Aerospike cluster from the configuration given its name.
// @Summary     Reads a specific Aerospike cluster from the configuration given its name.
// @ID	        readCluster
// @Tags        Configuration
// @Router      /v1/config/clusters/{name} [get]
// @Param       name path string true "Aerospike cluster name"
// @Produce     json
// @Success  	200 {object} model.AerospikeCluster
// @Response    400 {string} string
// @Failure     404 {string} string "The specified cluster could not be found"
func (s *Service) readAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "readAerospikeCluster"))

	clusterName := r.PathValue("name")
	if clusterName == "" {
		hLogger.Error("cluster name required")
		http.Error(w, clusterNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	cluster, ok := s.config.AerospikeClusters[clusterName]
	if !ok {
		hLogger.Error("cluster not found",
			slog.String("name", clusterName),
		)
		http.Error(w, fmt.Sprintf("cluster %s could not be found", clusterName), http.StatusNotFound)
		return
	}
	jsonResponse, err := json.Marshal(cluster)
	if err != nil {
		hLogger.Error("failed to marshal cluster",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		hLogger.Error("failed to write response",
			slog.String("response", string(jsonResponse)),
			slog.Any("error", err),
		)
		slog.Error("failed to write response", "err", err)
	}
}

// updateAerospikeCluster updates an existing Aerospike cluster in the configuration.
// @Summary     Updates an existing Aerospike cluster in the configuration.
// @ID	        updateCluster
// @Tags        Configuration
// @Router      /v1/config/clusters/{name} [put]
// @Accept      json
// @Param       name path string true "Aerospike cluster name"
// @Param       cluster body model.AerospikeCluster true "Aerospike cluster details"
// @Success     200
// @Failure     400 {string} string
func (s *Service) updateAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "updateAerospikeCluster"))

	var updatedCluster model.AerospikeCluster
	err := json.NewDecoder(r.Body).Decode(&updatedCluster)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()
	clusterName := r.PathValue("name")
	if clusterName == "" {
		hLogger.Error("cluster name required")
		http.Error(w, clusterNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	err = service.UpdateCluster(s.config, clusterName, &updatedCluster)
	if err != nil {
		hLogger.Error("failed to update cluster",
			slog.String("name", clusterName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = s.configurationManager.WriteConfiguration(s.config)
	if err != nil {
		hLogger.Error("failed to write configuration",
			slog.String("name", clusterName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	w.WriteHeader(http.StatusOK)
}

// deleteAerospikeCluster
// @Summary     Deletes a cluster from the configuration by name.
// @ID          deleteCluster
// @Tags        Configuration
// @Router      /v1/config/clusters/{name} [delete]
// @Param       name path string true "Aerospike cluster name"
// @Success     204
// @Failure     400 {string} string
//
//nolint:dupl // Each handler must be in separate func. No duplication.
func (s *Service) deleteAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "deleteAerospikeCluster"))

	clusterName := r.PathValue("name")
	if clusterName == "" {
		hLogger.Error("cluster name required")
		http.Error(w, clusterNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}
	err := service.DeleteCluster(s.config, clusterName)
	if err != nil {
		hLogger.Error("failed to delete cluster",
			slog.String("name", clusterName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = s.configurationManager.WriteConfiguration(s.config)
	if err != nil {
		hLogger.Error("failed to write configuration",
			slog.String("name", clusterName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
