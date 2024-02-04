package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"github.com/reugn/go-quartz/quartz"
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
	listFunc := func(timeBounds *model.TimeBounds, backend service.BackupListReader) ([]model.BackupDetails, error) {
		return backend.FullBackupList(timeBounds)
	}
	ws.getAvailableBackups(w, r, listFunc)
}

// @Summary  Get available incremental backups.
// @ID       getAvailableIncrementalBackups
// @Tags     Backup
// @Produce  json
// @Param    name query string false "Backup routine name"
// @Param    from query int false "Lower bound timestamp filter" format(int64)
// @Param    to query int false "Upper bound timestamp filter" format(int64)
// @Router   /backup/incremental/list [get]
// @Success  200 {object} map[string][]model.BackupDetails "Incremental backups by routine"
// @Failure  404 {string} string ""
func (ws *HTTPServer) getAvailableIncrementalBackups(w http.ResponseWriter, r *http.Request) {
	ws.getAvailableBackups(w, r, func(timeBounds *model.TimeBounds, backend service.BackupListReader) ([]model.BackupDetails, error) {
		return backend.IncrementalBackupList(timeBounds)
	})
}

func (ws *HTTPServer) getAvailableBackups(
	w http.ResponseWriter,
	r *http.Request,
	backupListFunc func(timebound *model.TimeBounds, backend service.BackupListReader) ([]model.BackupDetails, error)) {

	routines := ws.requestedRoutines(r)
	timeBounds, err := model.NewTimeBoundsFromString(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		http.Error(w, "failed parse time limits: "+err.Error(), http.StatusBadRequest)
		return
	}
	routineToBackups := make(map[string][]model.BackupDetails)
	for _, routine := range routines {
		backend, exists := ws.backupBackends[routine]
		if !exists {
			http.Error(w, "routine does not exist for "+routine, http.StatusNotFound)
			return
		}
		list, err := backupListFunc(timeBounds, backend)
		if err != nil {
			http.Error(w, "failed to retrieve backup list: "+err.Error(), http.StatusInternalServerError)
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
		slog.Error("failed to write response", "err", err)
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

// @Summary  Schedule a full backup once per routine name.
// @ID       scheduleFullBackup
// @Tags     Backup
// @Param    name query string true "Backup routine name"
// @Param    delay query int false "Delay interval in milliseconds"
// @Router   /backup/schedule [post]
// @Success  202
// @Failure  404 {string} string ""
func (ws *HTTPServer) scheduleFullBackup(w http.ResponseWriter, r *http.Request) {
	routineName := r.URL.Query().Get("name")
	delayParameter := r.URL.Query().Get("delay")
	var delayMillis int
	if delayParameter != "" {
		var err error
		delayMillis, err = strconv.Atoi(delayParameter)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if delayMillis < 0 {
		http.Error(w, "nonpositive delay query parameter", http.StatusBadRequest)
		return
	}
	fullBackupJobDetail := service.NewAdHocFullBackupJobForRoutine(routineName)
	if fullBackupJobDetail == nil {
		http.Error(w, "unknown routine name", http.StatusNotFound)
		return
	}
	trigger := quartz.NewRunOnceTrigger(time.Duration(delayMillis) * time.Millisecond)
	// schedule using the quartz scheduler
	if err := ws.scheduler.ScheduleJob(fullBackupJobDetail, trigger); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
