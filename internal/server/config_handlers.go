package server

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
// @Router      /config [get]
// @Produce     json
// @Success     200 {array} model.Config
// @Failure     400 {string} string
func (ws *HTTPServer) readConfig(w http.ResponseWriter) {
	configuration, err := json.MarshalIndent(ws.config, "", "    ") // pretty print
	if err != nil {
		http.Error(w, "Failed to parse service configuration", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(configuration)
}

// updateConfig
// @Summary     Updates the configuration for the service.
// @ID 	        updateConfig
// @Tags        Configuration
// @Router      /config [post]
// @Accept      json
// @Param       storage body model.Config true "config"
// @Success     200
// @Failure     400 {string} string
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
	w.WriteHeader(http.StatusOK)
}

// addAerospikeCluster
// @Summary     Adds an Aerospike cluster to the config.
// @ID          addCluster
// @Tags        Configuration
// @Router      /config/cluster [post]
// @Accept      json
// @Param       cluster body model.AerospikeCluster true "cluster info"
// @Success     201
// @Failure     400 {string} string
func (ws *HTTPServer) addAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	var newCluster model.AerospikeCluster
	err := json.NewDecoder(r.Body).Decode(&newCluster)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = service.AddCluster(ws.config, &newCluster)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = ConfigurationManager.WriteConfiguration(ws.config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	w.WriteHeader(http.StatusCreated)
}

// readAerospikeClusters reads all Aerospike clusters from the configuration.
// @Summary     Reads all Aerospike clusters from the configuration.
// @ID	        readClusters
// @Tags        Configuration
// @Router      /config/cluster [get]
// @Produce     json
// @Success     200 {array} model.AerospikeCluster
// @Failure     400 {string} string
func (ws *HTTPServer) readAerospikeClusters(w http.ResponseWriter) {
	clusters := ws.config.AerospikeClusters
	jsonResponse, err := json.Marshal(clusters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(jsonResponse)
}

// updateAerospikeCluster updates an existing Aerospike cluster in the configuration.
// @Summary     Updates an existing Aerospike cluster in the configuration.
// @ID	        updateCluster
// @Tags        Configuration
// @Router      /config/cluster [put]
// @Accept      json
// @Param       storage body model.AerospikeCluster true "aerospike cluster"
// @Success     200
// @Failure     400 {string} string
func (ws *HTTPServer) updateAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	var updatedCluster model.AerospikeCluster
	err := json.NewDecoder(r.Body).Decode(&updatedCluster)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = service.UpdateCluster(ws.config, &updatedCluster)
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
// @Router      /config/cluster [delete]
// @Param       name query string true "Cluster Name"
// @Success     204
// @Failure     400 {string} string
func (ws *HTTPServer) deleteAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	clusterName := r.URL.Query().Get("name")
	if clusterName == "" {
		http.Error(w, "Cluster name is required", http.StatusBadRequest)
		return
	}

	err := service.DeleteCluster(ws.config, &clusterName)
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

// addStorage
// @Summary     Adds a storage cluster to the config.
// @ID	        addStorage
// @Tags        Configuration
// @Router      /config/storage [post]
// @Accept      json
// @Param       storage body model.BackupStorage true "backup storage"
// @Success     201
// @Failure     400 {string} string
func (ws *HTTPServer) addStorage(w http.ResponseWriter, r *http.Request) {
	var newStorage model.BackupStorage
	err := json.NewDecoder(r.Body).Decode(&newStorage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = service.AddStorage(ws.config, &newStorage)
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

// readStorages reads all storages from the configuration.
// @Summary     Reads all storages from the configuration.
// @ID 	        readStorages
// @Tags        Configuration
// @Router      /config/storage [get]
// @Produce     json
// @Success     200 {array} model.BackupStorage
// @Failure     400 {string} string
func (ws *HTTPServer) readStorages(w http.ResponseWriter) {
	storage := ws.config.BackupStorage
	jsonResponse, err := json.Marshal(storage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(jsonResponse)
}

// updateStorage updates an existing storage in the configuration.
// @Summary     Updates an existing storage in the configuration.
// @ID	        updateStorage
// @Tags        Configuration
// @Router      /config/storage [put]
// @Accept      json
// @Param       storage body model.BackupStorage true "backup storage"
// @Success     200
// @Failure     400 {string} string
func (ws *HTTPServer) updateStorage(w http.ResponseWriter, r *http.Request) {
	var updatedStorage model.BackupStorage
	err := json.NewDecoder(r.Body).Decode(&updatedStorage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = service.UpdateStorage(ws.config, &updatedStorage)
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
// @Param       name query string true "Storage Name"
// @Success     204
// @Failure     400 {string} string
func (ws *HTTPServer) deleteStorage(w http.ResponseWriter, r *http.Request) {
	storageName := r.URL.Query().Get("name")
	if storageName == "" {
		http.Error(w, "Storage name is required", http.StatusBadRequest)
		return
	}

	err := service.DeleteStorage(ws.config, &storageName)
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

// addPolicy
// @Summary     Adds a policy to the config.
// @ID          addPolicy
// @Tags        Configuration
// @Router      /config/policy [post]
// @Accept      json
// @Param       storage body model.BackupPolicy true "backup policy"
// @Success     201
// @Failure     400 {string} string
func (ws *HTTPServer) addPolicy(w http.ResponseWriter, r *http.Request) {
	var newPolicy model.BackupPolicy
	err := json.NewDecoder(r.Body).Decode(&newPolicy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = service.AddPolicy(ws.config, &newPolicy)
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

// readPolicies reads all backup policies from the configuration.
// @Summary     Reads all policies from the configuration.
// @ID	        readPolicies
// @Tags        Configuration
// @Router      /config/policy [get]
// @Produce     json
// @Success     200 {array} model.BackupPolicy
// @Failure     400 {string} string
func (ws *HTTPServer) readPolicies(w http.ResponseWriter) {
	policies := ws.config.BackupPolicy
	jsonResponse, err := json.Marshal(policies)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(jsonResponse)
}

// updatePolicy updates an existing policy in the configuration.
// @Summary     Updates an existing policy in the configuration.
// @ID 	        updatePolicy
// @Tags        Configuration
// @Router      /config/policy [put]
// @Accept      json
// @Param       storage body model.BackupPolicy true "backup policy"
// @Success     200
// @Failure     400 {string} string
func (ws *HTTPServer) updatePolicy(w http.ResponseWriter, r *http.Request) {
	var updatedPolicy model.BackupPolicy
	err := json.NewDecoder(r.Body).Decode(&updatedPolicy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = service.UpdatePolicy(ws.config, &updatedPolicy)
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

// deletePolicy
// @Summary     Deletes a policy from the configuration by name.
// @ID          deletePolicy
// @Tags        Configuration
// @Router      /config/policy [delete]
// @Param       name query string true "Policy Name"
// @Success     204
// @Failure     400 {string} string
func (ws *HTTPServer) deletePolicy(w http.ResponseWriter, r *http.Request) {
	policyName := r.URL.Query().Get("name")
	if policyName == "" {
		http.Error(w, "Policy name is required", http.StatusBadRequest)
		return
	}

	err := service.DeletePolicy(ws.config, &policyName)
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
