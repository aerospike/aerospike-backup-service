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
// @Success     200 {object} model.Config
// @Failure     400 {string} string
func (ws *HTTPServer) readConfig(w http.ResponseWriter) {
	configuration, err := json.MarshalIndent(ws.config, "", "    ") // pretty print
	if err != nil {
		http.Error(w, "Failed to parse service configuration", http.StatusInternalServerError)
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
// @Router      /config [post]
// @Accept      json
// @Param       config body model.Config true "config"
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
// @Param       name query string true "cluster name"
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
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "cluster name is required", http.StatusBadRequest)
		return
	}
	err = service.AddCluster(ws.config, &name, &newCluster)
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
// @Success  	200 {object} map[string]model.AerospikeCluster
// @Failure     400 {string} string
func (ws *HTTPServer) readAerospikeClusters(w http.ResponseWriter) {
	clusters := ws.config.AerospikeClusters
	jsonResponse, err := json.Marshal(clusters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jsonResponse)
}

// updateAerospikeCluster updates an existing Aerospike cluster in the configuration.
// @Summary     Updates an existing Aerospike cluster in the configuration.
// @ID	        updateCluster
// @Tags        Configuration
// @Router      /config/cluster [put]
// @Accept      json
// @Param       name query string true "cluster name"
// @Param       cluster body model.AerospikeCluster true "aerospike cluster"
// @Success     200
// @Failure     400 {string} string
func (ws *HTTPServer) updateAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	var updatedCluster model.AerospikeCluster
	err := json.NewDecoder(r.Body).Decode(&updatedCluster)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "cluster name is required", http.StatusBadRequest)
		return
	}
	err = service.UpdateCluster(ws.config, &name, &updatedCluster)
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
// @Param       name query string true "cluster Name"
// @Success     204
// @Failure     400 {string} string
func (ws *HTTPServer) deleteAerospikeCluster(w http.ResponseWriter, r *http.Request) {
	clusterName := r.URL.Query().Get("name")
	if clusterName == "" {
		http.Error(w, "cluster name is required", http.StatusBadRequest)
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
	err = service.AddStorage(ws.config, &name, &newStorage)
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
// @ID 	        readStorage
// @Tags        Configuration
// @Router      /config/storage [get]
// @Produce     json
// @Success  	200 {object} map[string]model.Storage
// @Failure     400 {string} string
func (ws *HTTPServer) readStorage(w http.ResponseWriter) {
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
	err = service.UpdateStorage(ws.config, &name, &updatedStorage)
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
// @Param       name query string true "policy name"
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
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "policy name is required", http.StatusBadRequest)
		return
	}
	err = service.AddPolicy(ws.config, &name, &newPolicy)
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
// @Success  	200 {object} map[string]model.BackupPolicy
// @Failure     400 {string} string
func (ws *HTTPServer) readPolicies(w http.ResponseWriter) {
	jsonResponse, err := json.Marshal(ws.config.BackupPolicies)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jsonResponse)
}

// updatePolicy updates an existing policy in the configuration.
// @Summary     Updates an existing policy in the configuration.
// @ID 	        updatePolicy
// @Tags        Configuration
// @Router      /config/policy [put]
// @Accept      json
// @Param       name query string true "policy name"
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
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "policy name is required", http.StatusBadRequest)
		return
	}
	err = service.UpdatePolicy(ws.config, &name, &updatedPolicy)
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

// addRoutine
// @Summary     Adds a backup routine to the config.
// @ID          addRoutine
// @Tags        Configuration
// @Router      /config/routine [post]
// @Accept      json
// @Param       name query string true "routine name"
// @Param       storage body model.BackupRoutine true "backup routine"
// @Success     201
// @Failure     400 {string} string
func (ws *HTTPServer) addRoutine(w http.ResponseWriter, r *http.Request) {
	var newRoutine model.BackupRoutine
	err := json.NewDecoder(r.Body).Decode(&newRoutine)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "routine name is required", http.StatusBadRequest)
		return
	}
	err = service.AddRoutine(ws.config, &name, &newRoutine)
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

// readRoutines reads all backup routines from the configuration.
// @Summary     Reads all routines from the configuration.
// @ID	        readRoutines
// @Tags        Configuration
// @Router      /config/routine [get]
// @Produce     json
// @Success  	200 {object} map[string]model.BackupRoutine
// @Failure     400 {string} string
func (ws *HTTPServer) readRoutines(w http.ResponseWriter) {
	jsonResponse, err := json.Marshal(ws.config.BackupRoutines)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jsonResponse)
}

// updateRoutine updates an existing backup routine in the configuration.
// @Summary      Updates an existing routine in the configuration.
// @ID 	         updateRoutine
// @Tags         Configuration
// @Router       /config/routine [put]
// @Accept       json
// @Param        name query string true "routine name"
// @Param        storage body model.BackupRoutine true "backup routine"
// @Success      200
// @Failure      400 {string} string
func (ws *HTTPServer) updateRoutine(w http.ResponseWriter, r *http.Request) {
	var updatedRoutine model.BackupRoutine
	err := json.NewDecoder(r.Body).Decode(&updatedRoutine)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "routine name is required", http.StatusBadRequest)
		return
	}
	err = service.UpdateRoutine(ws.config, &name, &updatedRoutine)
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

// deleteRoutine
// @Summary     Deletes a backup routine from the configuration by name.
// @ID          deleteRoutine
// @Tags        Configuration
// @Router      /config/routine [delete]
// @Param       name query string true "routine name"
// @Success     204
// @Failure     400 {string} string
func (ws *HTTPServer) deleteRoutine(w http.ResponseWriter, r *http.Request) {
	routineName := r.URL.Query().Get("name")
	if routineName == "" {
		http.Error(w, "Routine name is required", http.StatusBadRequest)
		return
	}

	err := service.DeleteRoutine(ws.config, &routineName)
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
