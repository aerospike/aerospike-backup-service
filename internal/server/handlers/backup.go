package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service"
	"github.com/gorilla/mux"
	"github.com/reugn/go-quartz/quartz"
)

// GetAllFullBackups
// @Summary  Get available full backups.
// @ID 	     getFullBackups
// @Tags     Backup
// @Produce  json
// @Param    from query int false "Lower bound timestamp filter" format(int64)
// @Param    to query int false "Upper bound timestamp filter" format(int64)
// @Router   /v1/backups/full [get]
// @Success  200 {object} map[string][]dto.BackupDetails "Full backups by routine"
// @Failure  400 {string} string
// @Failure  500 {string} string
func (s *Service) GetAllFullBackups(w http.ResponseWriter, r *http.Request) {
	s.readAllBackups(w, r, true)
}

// GetFullBackupsForRoutine
// @Summary  Get available full backups for routine.
// @ID 	     GetFullBackupsForRoutine
// @Tags     Backup
// @Produce  json
// @Param    name path string true "Backup routine name"
// @Param    from query int false "Lower bound timestamp filter" format(int64)
// @Param    to query int false "Upper bound timestamp filter" format(int64)
// @Router   /v1/backups/full/{name} [get]
// @Success  200 {object} []dto.BackupDetails "Full backups for routine"
// @Failure  400 {string} string
// @Failure  500 {string} string
func (s *Service) GetFullBackupsForRoutine(w http.ResponseWriter, r *http.Request) {
	s.readBackupsForRoutine(w, r, true)
}

// GetAllIncrementalBackups
// @Summary  Get available incremental backups.
// @ID       getIncrementalBackups
// @Tags     Backup
// @Produce  json
// @Param    from query int false "Lower bound timestamp filter" format(int64)
// @Param    to query int false "Upper bound timestamp filter" format(int64)
// @Router   /v1/backups/incremental [get]
// @Success  200 {object} map[string][]dto.BackupDetails "Incremental backups by routine"
// @Failure  400 {string} string
// @Failure  500 {string} string
func (s *Service) GetAllIncrementalBackups(w http.ResponseWriter, r *http.Request) {
	s.readAllBackups(w, r, false)
}

// GetIncrementalBackupsForRoutine
// @Summary  Get incremental backups for routine.
// @ID       GetIncrementalBackupsForRoutine
// @Tags     Backup
// @Produce  json
// @Param    name path string true "Backup routine name"
// @Param    from query int false "Lower bound timestamp filter" format(int64)
// @Param    to query int false "Upper bound timestamp filter" format(int64)
// @Router   /v1/backups/incremental/{name} [get]
// @Success  200 {object} []dto.BackupDetails "Incremental backups for routine"
// @Failure  400 {string} string
// @Failure  500 {string} string
func (s *Service) GetIncrementalBackupsForRoutine(w http.ResponseWriter, r *http.Request) {
	s.readBackupsForRoutine(w, r, false)
}

func (s *Service) readAllBackups(w http.ResponseWriter, r *http.Request, isFullBackup bool) {
	hLogger := s.logger.With(slog.String("handler", "readAllBackups"))

	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	timeBounds, err := dto.NewTimeBoundsFromString(from, to)
	if err != nil {
		hLogger.Error("failed parse time limits",
			slog.String("from", from),
			slog.String("to", to),
			slog.Any("error", err),
		)
		http.Error(w, "failed parse time limits: "+err.Error(), http.StatusBadRequest)
		return
	}
	backups, err := readBackupsLogic(s.config.BackupRoutines, s.backupBackends, timeBounds.ToModel(), isFullBackup)
	if err != nil {
		hLogger.Error("failed to retrieve backup list",
			slog.Any("timeBounds", timeBounds),
			slog.Bool("isFullBackup", isFullBackup),
			slog.Any("error", err),
		)
		http.Error(w, "failed to retrieve backup list: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response, err := dto.Serialize(dto.ConvertBackupDetailsMap(backups), dto.JSON)
	if err != nil {
		hLogger.Error("failed to marshal backup list",
			slog.Any("error", err),
		)
		http.Error(w, "failed to parse backup list", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(response)
	if err != nil {
		hLogger.Error("failed to write response",
			slog.String("response", string(response)),
			slog.Any("error", err),
		)
	}
}

//nolint:funlen // Function is long because of logging.
func (s *Service) readBackupsForRoutine(w http.ResponseWriter, r *http.Request, isFullBackup bool) {
	hLogger := s.logger.With(slog.String("handler", "readBackupsForRoutine"))

	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	timeBounds, err := dto.NewTimeBoundsFromString(from, to)
	if err != nil {
		hLogger.Error("failed parse time limits",
			slog.String("from", from),
			slog.String("to", to),
			slog.Any("error", err),
		)
		http.Error(w, "failed parse time limits: "+err.Error(), http.StatusBadRequest)
		return
	}

	routine := mux.Vars(r)["name"]
	if routine == "" {
		hLogger.Error("routine name required")
		http.Error(w, "routine name required", http.StatusBadRequest)
		return
	}

	reader, found := s.backupBackends.GetReader(routine)
	if !found {
		hLogger.Error("routine name not found",
			slog.String("routine", routine),
		)
		http.Error(w, "routine name not found: "+routine, http.StatusBadRequest)
		return
	}

	backupListFunction := backupsReadFunction(reader, isFullBackup)
	backups, err := backupListFunction(r.Context(), timeBounds.ToModel())
	if err != nil {
		hLogger.Error("failed to retrieve backup list",
			slog.Bool("isFullBackup", isFullBackup),
			slog.Any("timeBounds", timeBounds),
			slog.Any("error", err),
		)
		http.Error(w, "failed to retrieve backup list: "+err.Error(), http.StatusInternalServerError)
		return
	}
	backupDetails := dto.ConvertModelsToDTO(backups, dto.NewBackupDetailsFromModel)
	response, err := dto.Serialize(backupDetails, dto.JSON)
	if err != nil {
		hLogger.Error("failed to marshal backup list",
			slog.Any("error", err),
		)
		http.Error(w, "failed to marshal backup list", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(response)
	if err != nil {
		hLogger.Error("failed to write response",
			slog.String("response", string(response)),
			slog.Any("error", err),
		)
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
		list, err := backupListFunction(context.Background(), timeBounds)
		if err != nil {
			return nil, err
		}
		result[routine] = list
	}
	return result, nil
}

func backupsReadFunction(
	backend service.BackupListReader, fullBackup bool,
) func(context.Context, *model.TimeBounds) ([]model.BackupDetails, error) {
	if fullBackup {
		return backend.FullBackupList
	}
	return backend.IncrementalBackupList
}

// ScheduleFullBackup
// @Summary  Schedule a full backup once per routine name.
// @ID       ScheduleFullBackup
// @Tags     Backup
// @Param    name path string true "Backup routine name"
// @Param    delay query int false "Delay interval in milliseconds"
// @Router   /v1/backups/schedule/{name} [post]
// @Success  202
// @Failure  400 {string} string
// @Failure  404 {string} string
// @Failure  500 {string} string
func (s *Service) ScheduleFullBackup(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "ScheduleFullBackup"))

	routineName := mux.Vars(r)["name"]
	if routineName == "" {
		hLogger.Error("routine name required")
		http.Error(w, "routine name required", http.StatusBadRequest)
		return
	}
	delayParameter := r.URL.Query().Get("delay")
	var delayMillis int
	if delayParameter != "" {
		var err error
		delayMillis, err = strconv.Atoi(delayParameter)
		if err != nil {
			hLogger.Error("failed to parse delay parameter",
				slog.String("delayParameter", delayParameter),
				slog.Any("error", err),
			)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if delayMillis < 0 {
		hLogger.Error("nonpositive delay query parameter",
			slog.Int("delayMillis", delayMillis),
		)
		http.Error(w, "nonpositive delay query parameter", http.StatusBadRequest)
		return
	}
	fullBackupJobDetail := service.NewAdHocFullBackupJobForRoutine(routineName)
	if fullBackupJobDetail == nil {
		hLogger.Error("unknown routine name",
			slog.String("name", routineName),
		)
		http.Error(w, "unknown routine name "+routineName, http.StatusNotFound)
		return
	}
	trigger := quartz.NewRunOnceTrigger(time.Duration(delayMillis) * time.Millisecond)
	// schedule using the quartz scheduler
	if err := s.scheduler.ScheduleJob(fullBackupJobDetail, trigger); err != nil {
		hLogger.Error("failed to schedule job",
			slog.Any("trigger", trigger),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// GetCurrentBackupInfo
// @Summary  Get current backup statistics.
// @ID       getCurrentBackup
// @Tags     Backup
// @Produce  json
// @Param    name path string true "Backup routine name"
// @Router   /v1/backups/currentBackup/{name} [get]
// @Success  200 {object} dto.CurrentBackups "Current backup statistics"
// @Failure  404 {string} string
// @Failure  400 {string} string
// @Failure  500 {string} string
func (s *Service) GetCurrentBackupInfo(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "GetCurrentBackupInfo"))

	routineName := mux.Vars(r)["name"]
	if routineName == "" {
		hLogger.Error("routine name required")
		http.Error(w, "routine name required", http.StatusBadRequest)
		return
	}

	handler, found := s.handlerHolder[routineName]
	if !found {
		hLogger.Error("unknown routine name",
			slog.String("name", routineName),
		)
		http.Error(w, "unknown routine name "+routineName, http.StatusNotFound)
		return
	}

	currentBackups := dto.NewCurrentBackupsFromModel(handler.GetCurrentStat())
	response, err := dto.Serialize(currentBackups, dto.JSON)
	if err != nil {
		hLogger.Error("failed to marshal statistics",
			slog.Any("error", err),
		)
		http.Error(w, "failed to marshal statistics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(response)
	if err != nil {
		hLogger.Error("failed to write response",
			slog.String("response", string(response)),
			slog.Any("error", err),
		)
	}
}
