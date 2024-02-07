package server

import (
	"encoding/json"
	"fmt"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"log/slog"
	"net/http"
)

// addStorage
// @Summary     Adds a storage cluster to the config.
// @ID	        addStorage
// @Tags        Configuration
// @Router      /config/storage [post]
// @Accept      json
// @Param       name query string true "storage name"
// @Param       storage body model.Storage true "backup storage"
// @Success     201
// @Failure     400 {string} string
func (ws *HTTPServer) addStorage(w http.ResponseWriter, r *http.Request) {
	var newStorage model.Storage
	err := json.NewDecoder(r.Body).Decode(&newStorage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "storage name is required", http.StatusBadRequest)
		return
	}
	err = service.AddStorage(ws.config, name, &newStorage)
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

// readStorage reads all storage from the configuration.
// @Summary     Reads all storage from the configuration.
// @ID 	        readAllStorage
// @Tags        Configuration
// @Router      /config/storages [get]
// @Produce     json
// @Success  	200 {object} map[string]model.Storage
// @Failure     400 {string} string
func (ws *HTTPServer) readStorages(w http.ResponseWriter, _ *http.Request) {
	storage := ws.config.Storage
	jsonResponse, err := json.Marshal(storage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jsonResponse)
}

// readAerospikeCluster reads a specific Aerospike cluster from the configuration given its name.
// @Summary     Reads an Aerospike cluster from the configuration based on its name.
// @ID	        readStorage
// @Tags        Configuration
// @Router      /config/storage [get]
// @Produce     json
// @Success  	200 {object} model.AerospikeCluster
// @Failure     404 {string} string "The specified cluster could not be found."
func (ws *HTTPServer) readStorage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method, method should be GET", http.StatusMethodNotAllowed)
		return
	}
	clusterName := r.URL.Query().Get("name")
	if clusterName == "" {
		http.Error(w, "The 'name' query parameter is required.", http.StatusBadRequest)
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

// updateStorage updates an existing storage in the configuration.
// @Summary     Updates an existing storage in the configuration.
// @ID	        updateStorage
// @Tags        Configuration
// @Router      /config/storage [put]
// @Accept      json
// @Param       name query string true "storage name"
// @Param       storage body model.Storage true "backup storage"
// @Success     200
// @Failure     400 {string} string
func (ws *HTTPServer) updateStorage(w http.ResponseWriter, r *http.Request) {
	var updatedStorage model.Storage
	err := json.NewDecoder(r.Body).Decode(&updatedStorage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "storage name is required", http.StatusBadRequest)
		return
	}
	err = service.UpdateStorage(ws.config, name, &updatedStorage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = ConfigurationManager.WriteConfiguration(ws.config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// deleteStorage
// @Summary     Deletes a storage from the configuration by name.
// @ID	        deleteStorage
// @Tags        Configuration
// @Router      /config/storage [delete]
// @Param       name query string true "storage name"
// @Success     204
// @Failure     400 {string} string
func (ws *HTTPServer) deleteStorage(w http.ResponseWriter, r *http.Request) {
	storageName := r.URL.Query().Get("name")
	if storageName == "" {
		http.Error(w, "storage name is required", http.StatusBadRequest)
		return
	}

	err := service.DeleteStorage(ws.config, storageName)
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
