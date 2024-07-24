package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"github.com/reugn/go-quartz/quartz"
)

// @Summary  Get available full backups.
// @ID 	     getFullBackups
// @Tags     Backup
// @Produce  json
// @Param    from query int false "Lower bound timestamp filter" format(int64)
// @Param    to query int false "Upper bound timestamp filter" format(int64)
// @Router   /v1/backups/full [get]
// @Success  200 {object} map[string][]model.BackupDetails "Full backups by routine"
// @Failure  400 {string} string
func (ws *HTTPServer) getAllFullBackups(w http.ResponseWriter, r *http.Request) {
	ws.readAllBackups(w, r, true)
}

// @Summary  Get available full backups for routine.
// @ID 	     getFullBackupsForRoutine
// @Tags     Backup
// @Produce  json
// @Param    name path string true "Backup routine name"
// @Param    from query int false "Lower bound timestamp filter" format(int64)
// @Param    to query int false "Upper bound timestamp filter" format(int64)
// @Router   /v1/backups/full/{name} [get]
// @Success  200 {object} []model.BackupDetails "Full backups for routine"
// @Response 400 {string} string
// @Failure  404 {string} string
func (ws *HTTPServer) getFullBackupsForRoutine(w http.ResponseWriter, r *http.Request) {
	ws.readBackupsForRoutine(w, r, true)
}

// @Summary  Get available incremental backups.
// @ID       getIncrementalBackups
// @Tags     Backup
// @Produce  json
// @Param    from query int false "Lower bound timestamp filter" format(int64)
// @Param    to query int false "Upper bound timestamp filter" format(int64)
// @Router   /v1/backups/incremental [get]
// @Success  200 {object} map[string][]model.BackupDetails "Incremental backups by routine"
// @Failure  400 {string} string
func (ws *HTTPServer) getAllIncrementalBackups(w http.ResponseWriter, r *http.Request) {
	ws.readAllBackups(w, r, false)
}

// @Summary  Get incremental backups for routine.
// @ID       getIncrementalBackupsForRoutine
// @Tags     Backup
// @Produce  json
// @Param    name path string true "Backup routine name"
// @Param    from query int false "Lower bound timestamp filter" format(int64)
// @Param    to query int false "Upper bound timestamp filter" format(int64)
// @Router   /v1/backups/incremental/{name} [get]
// @Success  200 {object} []model.BackupDetails "Incremental backups for routine"
// @Response 400 {string} string
// @Failure  404 {string} string
func (ws *HTTPServer) getIncrementalBackupsForRoutine(w http.ResponseWriter, r *http.Request) {
	ws.readBackupsForRoutine(w, r, false)
}

func (ws *HTTPServer) readAllBackups(w http.ResponseWriter, r *http.Request, isFullBackup bool) {
	timeBounds, err := model.NewTimeBoundsFromString(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		http.Error(w, "failed parse time limits: "+err.Error(), http.StatusBadRequest)
		return
	}
	backups, err := readBackupsLogic(ws.config.BackupRoutines, ws.backupBackends, timeBounds, isFullBackup)
	if err != nil {
		http.Error(w, "failed to retrieve backup list: "+err.Error(), http.StatusInternalServerError)
		return
	}
	response, err := json.Marshal(backups)
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

func (ws *HTTPServer) readBackupsForRoutine(w http.ResponseWriter, r *http.Request, isFullBackup bool) {
	timeBounds, err := model.NewTimeBoundsFromString(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		http.Error(w, "failed parse time limits: "+err.Error(), http.StatusBadRequest)
		return
	}
	routine := r.PathValue("name")
	if routine == "" {
		http.Error(w, "routine name required", http.StatusBadRequest)
		return
	}
	reader, found := ws.backupBackends.GetReader(routine)
	if !found {
		http.Error(w, "routine name not found: "+routine, http.StatusBadRequest)
		return
	}
	backupListFunction := backupsReadFunction(reader, isFullBackup)
	backups, err := backupListFunction(timeBounds)
	if err != nil {
		http.Error(w, "failed to retrieve backup list: "+err.Error(), http.StatusInternalServerError)
		return
	}
	response, err := json.Marshal(backups)
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

func readBackupsLogic(routines map[string]*model.BackupRoutine,
	backends service.BackendsHolder,
	timeBounds *model.TimeBounds,
	isFullBackup bool) (map[string][]model.BackupDetails, error) {

	result := make(map[string][]model.BackupDetails)
	for routine := range routines {
		reader, _ := backends.GetReader(routine)
		backupListFunction := backupsReadFunction(reader, isFullBackup)
		list, err := backupListFunction(timeBounds)
		if err != nil {
			return nil, err
		}
		result[routine] = list
	}
	return result, nil
}

func backupsReadFunction(
	backend service.BackupListReader, fullBackup bool) func(*model.TimeBounds) ([]model.BackupDetails, error) {

	if fullBackup {
		return backend.FullBackupList
	}
	return backend.IncrementalBackupList
}

// @Summary  Schedule a full backup once per routine name.
// @ID       scheduleFullBackup
// @Tags     Backup
// @Param    name path string true "Backup routine name"
// @Param    delay query int false "Delay interval in milliseconds"
// @Router   /v1/backups/schedule/{name} [post]
// @Success  202
// @Response 400 {string} string
// @Failure  404 {string} string
func (ws *HTTPServer) scheduleFullBackup(w http.ResponseWriter, r *http.Request) {
	routineName := r.PathValue("name")
	if routineName == "" {
		http.Error(w, "routine name required", http.StatusBadRequest)
		return
	}
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

// @Summary  Get current backup statistics.
// @ID       getCurrentBackup
// @Tags     Backup
// @Produce  json
// @Param    name path string true "Backup routine name"
// @Router   /v1/backups/currentBackup/{name} [get]
// @Success  200 {object} int "Current backup statistics"
// @Failure  404 {string} string
func (ws *HTTPServer) getCurrentBackupInfo(w http.ResponseWriter, r *http.Request) {
	routineName := r.PathValue("name")
	if routineName == "" {
		http.Error(w, "routine name required", http.StatusBadRequest)
		return
	}

	handler := ws.handlerHolder.Handlers[routineName]
	stat := handler.GetCurrentStat()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%d", stat)
}
