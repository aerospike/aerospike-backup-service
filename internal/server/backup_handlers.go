package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
)

// @Summary  Get available full backups.
// @ID 	     getAvailableFullBackups
// @Tags     Backup
// @Produce  json
// @Param    name query string true "Backup routine name"
// @Router   /backup/full/list [get]
// @Success  200 {object} map[string][]model.BackupDetails "Full backups by routine"
// @Failure  404 {string} string ""
func (ws *HTTPServer) getAvailableFullBackups(w http.ResponseWriter, r *http.Request) {
	ws.getAvailableBackups(w, r, func(backend service.BackupBackend) ([]model.BackupDetails, error) {
		return backend.FullBackupList()
	})
}

// @Summary  Get available incremental backups.
// @ID       getAvailableIncrementalBackups
// @Tags     Backup
// @Produce  json
// @Param    name query string true "Backup routine name"
// @Router   /backup/incremental/list [get]
// @Success  200 {object} map[string][]model.BackupDetails "Incremental backups by routine"
// @Failure  404 {string} string ""
func (ws *HTTPServer) getAvailableIncrementalBackups(w http.ResponseWriter, r *http.Request) {
	ws.getAvailableBackups(w, r, func(backend service.BackupBackend) ([]model.BackupDetails, error) {
		return backend.IncrementalBackupList()
	})
}

func (ws *HTTPServer) getAvailableBackups(
	w http.ResponseWriter,
	r *http.Request,
	backupListFunc func(service.BackupBackend) ([]model.BackupDetails, error)) {

	routines := ws.requestedRoutines(r)
	routineToBackups := make(map[string][]model.BackupDetails)
	for _, routine := range routines {
		backend, exists := ws.backupBackends[routine]
		if !exists {
			http.Error(w, "Backup backend does not exist for "+routine, http.StatusNotFound)
			return
		}
		list, err := backupListFunc(backend)
		if err != nil {
			slog.Error("Failed to retrieve backup list", "err", err)
			http.Error(w, "Failed to retrieve backup list", http.StatusInternalServerError)
			return
		}
		routineToBackups[routine] = list
	}
	response, err := json.Marshal(routineToBackups)
	if err != nil {
		slog.Error("Failed to parse backup list", "err", err)
		http.Error(w, "Failed to parse backup list", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(response)
}

// return an array containing single routine from request (if present), or all routines
func (ws *HTTPServer) requestedRoutines(r *http.Request) []string {
	queryRoutineName := r.URL.Query().Get("name")
	if queryRoutineName != "" {
		return []string{queryRoutineName}
	}
	routines := make([]string, len(ws.config.BackupRoutines))
	for _, p := range ws.config.BackupRoutines {
		routines = append(routines, p.Name)
	}
	return routines
}
