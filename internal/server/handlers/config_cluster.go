package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/gorilla/mux"
)

const clusterNameNotSpecifiedMsg = "Cluster name is not specified"

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
	}
}

// addAerospikeCluster
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
func (s *Service) addAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "addAerospikeCluster"))

	newCluster, err := dto.NewClusterFromReader(r.Body, dto.JSON)
	if err != nil {
		hLogger.Error("failed to decode request body",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()

	name := mux.Vars(r)["name"]
	if name == "" {
		hLogger.Error(clusterNameNotSpecifiedMsg,
			slog.String("name", name),
		)
		http.Error(w, clusterNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.AddCluster(name, newCluster.ToModel())
	})
	if err != nil {
		hLogger.Error("failed to add cluster",
			slog.String("name", name),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	hLogger := s.logger.With(slog.String("handler", "ReadAerospikeClusters"))

	toDTO := dto.ConvertModelMapToDTO(s.config.AerospikeClusters, dto.NewClusterFromModel)
	jsonResponse, err := dto.Serialize(toDTO, dto.JSON)
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
// @Success  	200 {object} dto.AerospikeCluster
// @Failure     400 {string} string
// @Failure     404 {string} string "The specified cluster could not be found"
// @Failure     500 {string} string "The specified cluster could not be found"
func (s *Service) readAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "readAerospikeCluster"))

	clusterName := mux.Vars(r)["name"]
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
	jsonResponse, err := dto.Serialize(dto.NewClusterFromModel(cluster), dto.JSON)
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
// @Param       cluster body dto.AerospikeCluster true "Aerospike cluster details"
// @Success     200
// @Failure     400 {string} string
//
//nolint:dupl
func (s *Service) updateAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "updateAerospikeCluster"))

	updatedCluster, err := dto.NewClusterFromReader(r.Body, dto.JSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()
	clusterName := mux.Vars(r)["name"]
	if clusterName == "" {
		hLogger.Error("cluster name required")
		http.Error(w, clusterNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}

	err = s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.UpdateCluster(clusterName, updatedCluster.ToModel())
	})
	if err != nil {
		hLogger.Error("failed to update cluster",
			slog.String("name", clusterName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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
func (s *Service) deleteAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "deleteAerospikeCluster"))

	clusterName := mux.Vars(r)["name"]
	if clusterName == "" {
		hLogger.Error("cluster name required")
		http.Error(w, clusterNameNotSpecifiedMsg, http.StatusBadRequest)
		return
	}

	err := s.changeConfig(r.Context(), func(config *model.Config) error {
		return config.DeleteCluster(clusterName)
	})
	if err != nil {
		hLogger.Error("failed to delete cluster",
			slog.String("name", clusterName),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
