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
// @Param    name query string true "Backup policy name"
// @Router   /backup/full/list [get]
// @Success  200 {array} model.BackupDetails "Full backups"
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
// @Param    name query string true "Backup policy name"
// @Router   /backup/incremental/list [get]
// @Success  200 {array} model.BackupDetails "Incremental backups"
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

	policyName := r.URL.Query().Get("name")
	if policyName == "" {
		http.Error(w, "Undefined policy name", http.StatusBadRequest)
		return
	}

	backend, exists := ws.backupBackends[policyName]
	if !exists {
		http.Error(w, "Backup backend does not exist for "+policyName, http.StatusNotFound)
		return
	}

	list, err := backupListFunc(backend)
	if err != nil {
		slog.Error("Failed to retrieve backup list", "err", err)
		http.Error(w, "Failed to retrieve backup list", http.StatusInternalServerError)
		return
	}

	response, err := json.Marshal(list)
	if err != nil {
		slog.Error("Failed to parse backup list", "err", err)
		http.Error(w, "Failed to parse backup list", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(response)
}
