package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
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
	timeBounds, err := dto.NewTimeBoundsFromString(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		httpError(w, errInvalidQueryParam(err, "time bounds"))
		return
	}

	result, err := s.readAllBackups(r.Context(), func(routine string) service.BackupFilter {
		return service.NewFullBackupFilter(routine).WithTimebounds(timeBounds)
	})

	if err != nil {
		httpError(w, err)
		return
	}

	httpOK(w, result)
}

// GetFullBackupsForRoutine
// @Summary  Get available full backups for routine.
// @ID 	     getFullBackupsForRoutine
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
	timeBounds, err := dto.NewTimeBoundsFromString(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		httpError(w, errInvalidQueryParam(err, "time bounds"))
		return
	}

	routine := r.PathValue("name")
	if routine == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	filter := service.NewFullBackupFilter(routine).WithTimebounds(timeBounds)
	result, err := s.readBackupsForRoutine(r.Context(), filter)
	if err != nil {
		httpError(w, err)
		return
	}

	httpOK(w, result)
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
	timeBounds, err := dto.NewTimeBoundsFromString(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		httpError(w, errInvalidQueryParam(err, "time bounds"))
		return
	}

	result, err := s.readAllBackups(r.Context(), func(routine string) service.BackupFilter {
		return service.NewIncrementalBackupFilter(routine).WithTimebounds(timeBounds)
	})
	if err != nil {
		httpError(w, err)
		return
	}

	httpOK(w, result)
}

// GetIncrementalBackupsForRoutine
// @Summary  Get incremental backups for routine.
// @ID       getIncrementalBackupsForRoutine
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
	timeBounds, err := dto.NewTimeBoundsFromString(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		httpError(w, errInvalidQueryParam(err, "time bounds"))
		return
	}

	routine := r.PathValue("name")
	if routine == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	filter := service.NewIncrementalBackupFilter(routine).WithTimebounds(timeBounds)
	result, err := s.readBackupsForRoutine(r.Context(), filter)
	if err != nil {
		httpError(w, err)
		return
	}

	httpOK(w, result)
}

func (s *Service) readAllBackups(
	ctx context.Context,
	filter func(routine string) service.BackupFilter,
) (map[string][]*dto.BackupDetails, error) {
	result := make(map[string][]*dto.BackupDetails)
	for routine := range s.config.Routines() {
		routineBackups, err := s.readBackupsForRoutine(ctx, filter(routine))
		if err != nil {
			return nil, err
		}

		result[routine] = routineBackups
	}

	return result, nil
}

func (s *Service) readBackupsForRoutine(
	ctx context.Context,
	filter service.BackupFilter,
) ([]*dto.BackupDetails, error) {
	backupList, err := s.backendService.GetBackups(ctx, filter)
	if err != nil {
		return nil, err
	}

	backupConfig := s.config.BackupConfigCopy()
	backupDetails := dto.ConvertModelsToDTO(backupList, func(m *model.BackupDetails) *dto.BackupDetails {
		return dto.NewBackupDetailsFromModel(m, backupConfig)
	})

	return backupDetails, nil
}

// ScheduleFullBackup
// @Summary  Schedule a full backup once per routine name.
// @ID       scheduleFullBackup
// @Tags     Backup
// @Param    name path string true "Backup routine name"
// @Param    delay query int false "Delay interval in milliseconds"
// @Router   /v1/backups/schedule/{name} [post]
// @Success  202
// @Failure  400 {string} string
// @Failure  404 {string} string
// @Failure  500 {string} string
func (s *Service) ScheduleFullBackup(w http.ResponseWriter, r *http.Request) {
	routineName := r.PathValue("name")
	if routineName == "" {
		http.Error(w, "routine name required", http.StatusBadRequest)
		return
	}

	delayMillis, err := parseDelay(r.URL.Query().Get("delay"))
	if err != nil {
		httpError(w, err)
		return
	}

	fullBackupJobDetail := service.NewAdHocFullBackupJobForRoutine(routineName)
	if fullBackupJobDetail == nil {
		httpError(w, errRoutineNotFound(routineName))
		return
	}

	trigger := quartz.NewRunOnceTrigger(time.Duration(delayMillis) * time.Millisecond)
	// schedule using the quartz scheduler
	if err := s.scheduler.ScheduleJob(fullBackupJobDetail, trigger); err != nil {
		httpError(w, errors.New("failed to schedule job"))
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func parseDelay(delayParameter string) (int, error) {
	if delayParameter == "" {
		return 0, nil
	}

	delayMillis, err := strconv.Atoi(delayParameter)
	if err != nil || delayMillis < 0 {
		return 0, errInvalidQueryParam(errors.New("should be a positive integer"), "delay")
	}

	return delayMillis, nil
}

// GetCurrentBackupInfo
// @Summary  Get current backup statistics.
// @ID       getCurrentBackup
// @Tags     Backup
// @Produce  json
// @Param    name path string true "Backup routine name"
// @Router   /v1/backups/currentBackup/{name} [get]
// @Success  200 {object} dto.RoutineState "Current backup statistics"
// @Failure  404 {string} string
// @Failure  400 {string} string
// @Failure  500 {string} string
func (s *Service) GetCurrentBackupInfo(w http.ResponseWriter, r *http.Request) {
	routineName := r.PathValue("name")
	if routineName == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	_, found := s.config.Routine(routineName)
	if !found {
		httpError(w, errRoutineNotFound(routineName))
		return
	}

	currentBackups := dto.NewRoutineStateFromModel(s.registry.GetRoutineState(routineName))
	httpOK(w, currentBackups)
}

// CancelCurrentBackup
// @Summary  Cancel current backup.
// @ID       cancelCurrentBackup
// @Tags     Backup
// @Param    name path string true "Backup routine name"
// @Router   /v1/backups/cancel/{name} [post]
// @Success  202
// @Failure  404 {string} string
// @Failure  500 {string} string
func (s *Service) CancelCurrentBackup(w http.ResponseWriter, r *http.Request) {
	routineName := r.PathValue("name")
	if routineName == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	_, found := s.config.Routine(routineName)
	if !found {
		httpError(w, errRoutineNotFound(routineName))
		return
	}

	s.registry.Cancel(routineName)

	w.WriteHeader(http.StatusAccepted)
}
