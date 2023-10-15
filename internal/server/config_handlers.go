package server

import (
	"encoding/json"
	"fmt"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"net/http"
)

var ConfigurationManager service.ConfigurationManager

// readConfig
// @Summary Returns the configuration for the service.
// @Router /config [get]
// @Success 200 {array} model.Config
func (ws *HTTPServer) readConfig(w http.ResponseWriter) {
	configuration, err := json.MarshalIndent(ws.config, "", "    ") // pretty print
	if err != nil {
		http.Error(w, "Failed to parse service configuration", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(configuration)
}

// updateConfig
// @Summary Updates the configuration for the service.
// @Router /config [post]
// @Success 200 {array} model.Config
func (ws *HTTPServer) updateConfig(w http.ResponseWriter, r *http.Request) {
	var newConfig model.Config

	err := json.NewDecoder(r.Body).Decode(&newConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ws.config = &newConfig
	err = ConfigurationManager.WriteConfiguration(&newConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

// AddAerospikeCluster
// @Summary adds an Aerospike cluster to the config.
// @Router /config/cluster [post]
// @Success 200 ""
func (ws *HTTPServer) AddAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	var newCluster model.AerospikeCluster
	err := json.NewDecoder(r.Body).Decode(&newCluster)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, existingCluster := range ws.config.AerospikeClusters {
		if *existingCluster.Name == *newCluster.Name {
			errorMessage := fmt.Sprintf("Aerospike cluster with the same name %s already exists", *newCluster.Name)
			http.Error(w, errorMessage, http.StatusBadRequest)
			return
		}
	}

	ws.config.AerospikeClusters = append(ws.config.AerospikeClusters, &newCluster)
	ConfigurationManager.WriteConfiguration(ws.config)
}

// ReadAerospikeClusters reads all Aerospike clusters from the configuration.
// @Summary Reads all Aerospike clusters from the configuration.
// @Router /config/cluster [get]
// @Success 200 {array} model.AerospikeCluster
func (ws *HTTPServer) ReadAerospikeClusters(w http.ResponseWriter) {
	clusters := ws.config.AerospikeClusters
	jsonResponse, err := json.Marshal(clusters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResponse)
}

// UpdateAerospikeCluster updates an existing Aerospike cluster in the configuration.
// @Summary Updates an existing Aerospike cluster in the configuration.
// @Router /config/cluster [put]
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
func (ws *HTTPServer) UpdateAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	var updatedCluster model.AerospikeCluster
	err := json.NewDecoder(r.Body).Decode(&updatedCluster)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for i, cluster := range ws.config.AerospikeClusters {
		if *cluster.Name == *updatedCluster.Name {
			ws.config.AerospikeClusters[i] = &updatedCluster
			ConfigurationManager.WriteConfiguration(ws.config)
			return
		}
	}
	errorMessage := fmt.Sprintf("Cluster %s not found", *updatedCluster.Name)
	http.Error(w, errorMessage, http.StatusBadRequest)
}

// DeleteAerospikeCluster
// @Summary Deletes a cluster from the configuration by name.
// @Router /config/cluster [delete]
// @Param name query string true "Cluster Name"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
func (ws *HTTPServer) DeleteAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	clusterName := r.URL.Query().Get("name")
	if clusterName == "" {
		http.Error(w, "Cluster name is required", http.StatusBadRequest)
		return
	}

	for _, policy := range ws.config.BackupPolicy {
		if *policy.SourceCluster == clusterName {
			errorMessage := fmt.Sprintf("Cannot delete cluster as it is used in a policy %s", *policy.Name)
			http.Error(w, errorMessage, http.StatusBadRequest)
			return
		}
	}

	for i, cluster := range ws.config.AerospikeClusters {
		if *cluster.Name == clusterName {
			ws.config.AerospikeClusters = append(ws.config.AerospikeClusters[:i], ws.config.AerospikeClusters[i+1:]...)
			ConfigurationManager.WriteConfiguration(ws.config)
			return
		}
	}
	errorMessage := fmt.Sprintf("Cluster %s not found", clusterName)
	http.Error(w, errorMessage, http.StatusBadRequest)
}
