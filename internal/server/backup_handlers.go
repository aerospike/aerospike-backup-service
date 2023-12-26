package server

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
)

// @Summary  Get available full backups.
// @ID 	     getAvailableFullBackups
// @Tags     Backup
// @Produce  json
// @Param    name query string false "Backup routine name"
// @Param    from query int false "Lower bound timestamp filter" format(int64)
// @Param    to query int false "Upper bound timestamp filter" format(int64)
// @Router   /backup/full/list [get]
// @Success  200 {object} map[string][]model.BackupDetails "Full backups by routine"
// @Failure  404 {string} string ""
func (ws *HTTPServer) getAvailableFullBackups(w http.ResponseWriter, r *http.Request) {
	ws.getAvailableBackups(w, r, func(backend service.BackupBackend) ([]model.BackupDetails, error) {
		fromTime, err := parseTimestamp(r.URL.Query().Get("from"), 0)
		if err != nil {
			return nil, err
		}
		toTime, err := parseTimestamp(r.URL.Query().Get("to"), math.MaxInt64)
		if err != nil {
			return nil, err
		}
		return backend.FullBackupList(fromTime, toTime)
	})
}

// @Summary  Get available incremental backups.
// @ID       getAvailableIncrementalBackups
// @Tags     Backup
// @Produce  json
// @Param    name query string false "Backup routine name"
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
			http.Error(w, "backup backend does not exist for "+routine, http.StatusNotFound)
			return
		}
		list, err := backupListFunc(backend)
		if err != nil {
			http.Error(w, "failed to retrieve backup list", http.StatusInternalServerError)
			return
		}
		routineToBackups[routine] = list
	}
	response, err := json.Marshal(routineToBackups)
	if err != nil {
		http.Error(w, "failed to parse backup list", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(response)
	if err != nil {
		slog.Error("failed to write response", err)
	}
}

// return an array containing single routine from request (if present), or all routines
func (ws *HTTPServer) requestedRoutines(r *http.Request) []string {
	queryRoutineName := r.URL.Query().Get("name")
	if queryRoutineName != "" {
		return []string{queryRoutineName}
	}
	routines := make([]string, 0, len(ws.config.BackupRoutines))
	for name := range ws.config.BackupRoutines {
		routines = append(routines, name)
	}
	return routines
}

func parseTimestamp(value string, defaultValue int64) (int64, error) {
	if value == "" {
		return defaultValue, nil
	}
	return strconv.ParseInt(value, 10, 64)
}
