package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
)

// addRoutine
// @Summary     Adds a backup routine to the config.
// @ID          addRoutine
// @Tags        Configuration
// @Router      /config/routines/{name} [post]
// @Accept      json
// @Param       name path string true "routine name"
// @Param       storage body model.BackupRoutine true "backup routine"
// @Success     201
// @Failure     400 {string} string
//
//nolint:dupl
func (ws *HTTPServer) addRoutine(w http.ResponseWriter, r *http.Request) {
	var newRoutine model.BackupRoutine
	err := json.NewDecoder(r.Body).Decode(&newRoutine)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()
	name := getLastURLSegment(r.URL.Path)
	if name == "" {
		http.Error(w, "routine name is required", http.StatusBadRequest)
		return
	}
	err = service.AddRoutine(ws.config, name, &newRoutine)
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
// @Router      /config/routines [get]
// @Produce     json
// @Success  	200 {object} map[string]model.BackupRoutine
// @Failure     400 {string} string
func (ws *HTTPServer) readRoutines(w http.ResponseWriter, _ *http.Request) {
	jsonResponse, err := json.Marshal(ws.config.BackupRoutines)
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

// readRoutine reads a specific routine from the configuration given its name.
// @Summary     Reads a specific routine from the configuration given its name.
// @ID	        readRoutine
// @Tags        Configuration
// @Router      /config/routines/{name} [get]
// @Param       name path string true "Name of the routine"
// @Produce     json
// @Success  	200 {object} model.BackupRoutine
// @Failure     404 {string} string "The specified cluster could not be found."
func (ws *HTTPServer) readRoutine(w http.ResponseWriter, r *http.Request) {
	routineName := getLastURLSegment(r.URL.Path)
	if routineName == "" {
		http.Error(w, "The cluster name path parameter is required.", http.StatusBadRequest)
		return
	}

	routine, ok := ws.config.BackupRoutines[routineName]
	if !ok {
		http.Error(w, fmt.Sprintf("Routine %s could not be found.", routineName), http.StatusNotFound)
		return
	}

	jsonResponse, err := json.Marshal(routine)
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

// updateRoutine updates an existing backup routine in the configuration.
// @Summary      Updates an existing routine in the configuration.
// @ID 	         updateRoutine
// @Tags         Configuration
// @Router       /config/routines/{name} [put]
// @Accept       json
// @Param        name path string true "routine name"
// @Param        storage body model.BackupRoutine true "backup routine"
// @Success      200
// @Failure      400 {string} string
//
//nolint:dupl
func (ws *HTTPServer) updateRoutine(w http.ResponseWriter, r *http.Request) {
	var updatedRoutine model.BackupRoutine
	err := json.NewDecoder(r.Body).Decode(&updatedRoutine)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()
	name := getLastURLSegment(r.URL.Path)
	if name == "" {
		http.Error(w, "routine name is required", http.StatusBadRequest)
		return
	}
	err = service.UpdateRoutine(ws.config, name, &updatedRoutine)
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
// @Router      /config/routines/{name} [delete]
// @Param       name path string true "routine name"
// @Success     204
// @Failure     400 {string} string
func (ws *HTTPServer) deleteRoutine(w http.ResponseWriter, r *http.Request) {
	routineName := getLastURLSegment(r.URL.Path)
	if routineName == "" {
		http.Error(w, "routine name is required", http.StatusBadRequest)
		return
	}

	err := service.DeleteRoutine(ws.config, routineName)
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
