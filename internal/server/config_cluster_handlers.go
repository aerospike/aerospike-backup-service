package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
)

// addAerospikeCluster
// @Summary     Adds an Aerospike cluster to the config.
// @ID          addCluster
// @Tags        Configuration
// @Router      /v1/config/clusters/{name} [post]
// @Accept      json
// @Param       name path string true "cluster name"
// @Param       cluster body model.AerospikeCluster true "cluster info"
// @Success     201
// @Failure     400 {string} string
//
//nolint:dupl
func (ws *HTTPServer) addAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	var newCluster model.AerospikeCluster
	err := json.NewDecoder(r.Body).Decode(&newCluster)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "cluster name is required", http.StatusBadRequest)
		return
	}
	err = service.AddCluster(ws.config, name, &newCluster)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = ConfigurationManager.WriteConfiguration(ws.config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// readAerospikeClusters reads all Aerospike clusters from the configuration.
// @Summary     Reads all Aerospike clusters from the configuration.
// @ID	        readAllClusters
// @Tags        Configuration
// @Router      /v1/config/clusters [get]
// @Produce     json
// @Success  	200 {object} map[string]model.AerospikeCluster
// @Failure     400 {string} string
func (ws *HTTPServer) readAerospikeClusters(w http.ResponseWriter, _ *http.Request) {
	clusters := ws.config.AerospikeClusters
	jsonResponse, err := json.Marshal(clusters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		slog.Error("failed to write response", "err", err)
	}
}

// readAerospikeCluster reads a specific Aerospike cluster from the configuration given its name.
// @Summary     Reads a specific Aerospike cluster from the configuration given its name.
// @ID	        readCluster
// @Tags        Configuration
// @Router      /v1/config/clusters/{name} [get]
// @Param       name path string true "Name of the Aerospike cluster"
// @Produce     json
// @Success  	200 {object} model.AerospikeCluster
// @Failure     404 {string} string "The specified cluster could not be found."
func (ws *HTTPServer) readAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	clusterName := r.PathValue("name")
	if clusterName == "" {
		http.Error(w, "The cluster name path parameter is required.", http.StatusBadRequest)
		return
	}

	cluster, ok := ws.config.AerospikeClusters[clusterName]
	if !ok {
		http.Error(w, fmt.Sprintf("Cluster %s could not be found.", clusterName), http.StatusNotFound)
		return
	}

	jsonResponse, err := json.Marshal(cluster)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		slog.Error("failed to write response", "err", err)
	}
}

// updateAerospikeCluster updates an existing Aerospike cluster in the configuration.
// @Summary     Updates an existing Aerospike cluster in the configuration.
// @ID	        updateCluster
// @Tags        Configuration
// @Router      /v1/config/clusters/{name} [put]
// @Accept      json
// @Param       name path string true "cluster name"
// @Param       cluster body model.AerospikeCluster true "aerospike cluster"
// @Success     200
// @Failure     400 {string} string
//
//nolint:dupl
func (ws *HTTPServer) updateAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	var updatedCluster model.AerospikeCluster
	err := json.NewDecoder(r.Body).Decode(&updatedCluster)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()
	clusterName := r.PathValue("name")
	if clusterName == "" {
		http.Error(w, "cluster is required", http.StatusBadRequest)
		return
	}
	err = service.UpdateCluster(ws.config, clusterName, &updatedCluster)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = ConfigurationManager.WriteConfiguration(ws.config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	w.WriteHeader(http.StatusOK)
}

// deleteAerospikeCluster
// @Summary     Deletes a cluster from the configuration by name.
// @ID          deleteCluster
// @Tags        Configuration
// @Router      /v1/config/clusters/{name} [delete]
// @Param       name path string true "cluster Name"
// @Success     204
// @Failure     400 {string} string
func (ws *HTTPServer) deleteAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	clusterName := r.PathValue("name")
	if clusterName == "" {
		http.Error(w, "cluster name is required", http.StatusBadRequest)
		return
	}

	err := service.DeleteCluster(ws.config, clusterName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = ConfigurationManager.WriteConfiguration(ws.config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
