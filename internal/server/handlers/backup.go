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

// minAdHocBackupDelay is the minimum delay for ad-hoc backup triggers so the
// Quartz scheduler does not expire the trigger before the job runs.
const minAdHocBackupDelay = 50 * time.Millisecond

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

	result, err := s.readAllBackups(r.Context(), func(routine *model.BackupRoutine) service.BackupFilter {
		return service.NewFullBackupFilter(routine).WithTimeBounds(timeBounds)
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

	routineName := r.PathValue("name")
	if routineName == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	routine, found := s.config.Routine(routineName)
	if !found {
		httpError(w, errRoutineNotFound(routineName))
		return
	}

	filter := service.NewFullBackupFilter(routine).WithTimeBounds(timeBounds)
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

	result, err := s.readAllBackups(r.Context(), func(routine *model.BackupRoutine) service.BackupFilter {
		return service.NewIncrementalBackupFilter(routine).WithTimeBounds(timeBounds)
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

	routineName := r.PathValue("name")
	if routineName == "" {
		httpError(w, errMissingRoutineName)
		return
	}

	routine, found := s.config.Routine(routineName)
	if !found {
		httpError(w, errRoutineNotFound(routineName))
		return
	}

	filter := service.NewIncrementalBackupFilter(routine).WithTimeBounds(timeBounds)
	result, err := s.readBackupsForRoutine(r.Context(), filter)
	if err != nil {
		httpError(w, err)
		return
	}

	httpOK(w, result)
}

func (s *Service) readAllBackups(
	ctx context.Context,
	filter func(routine *model.BackupRoutine) service.BackupFilter,
) (map[string][]*dto.BackupDetails, error) {
	result := make(map[string][]*dto.BackupDetails)
	for _, routine := range s.config.Routines() {
		routineBackups, err := s.readBackupsForRoutine(ctx, filter(routine))
		if err != nil {
			if errors.Is(err, service.ErrNotFound) {
				continue // Not an error. Routine may have been deleted while reading backups for previous routines.
			}

			return nil, err
		}

		result[routine.Name] = routineBackups
	}

	return result, nil
}

func (s *Service) readBackupsForRoutine(
	ctx context.Context,
	filter service.BackupFilter,
) ([]*dto.BackupDetails, error) {
	backupList, err := s.backupReader.GetBackups(ctx, filter)
	if err != nil {
		return nil, err
	}

	backupConfig := s.config.BackupConfigCopy()
	backupDetails := dto.ConvertModelsToDTO(backupList, func(m *model.BackupDetails) *dto.BackupDetails {
		return dto.NewBackupDetailsFromModel(m, backupConfig)
	})

	return backupDetails, nil
}

// TriggerFullBackup
// @Summary  Trigger a full backup once per routine name.
// @ID       triggerFullBackup
// @Tags     Backup
// @Param    name path string true "Backup routine name"
// @Param    delay query int false "Delay interval in milliseconds"
// @Router   /v1/backups/full/{name} [post]
// @Success  202
// @Failure  400 {string} string
// @Failure  404 {string} string
// @Failure  500 {string} string
func (s *Service) TriggerFullBackup(w http.ResponseWriter, r *http.Request) {
	s.scheduleBackup(w, r, service.NewAdHocFullBackupJobForRoutine)
}

// ScheduleFullBackup
// @Summary     Schedule a full backup once per routine name.
// @Description Deprecated: use POST /v1/backups/full/{name} instead.
// @ID          scheduleFullBackup
// @Tags        Backup
// @Deprecated
// @Param       name path string true "Backup routine name"
// @Param       delay query int false "Delay interval in milliseconds"
// @Router      /v1/backups/schedule/{name} [post]
// @Success     202
// @Failure     400 {string} string
// @Failure     404 {string} string
// @Failure     500 {string} string
func (s *Service) ScheduleFullBackup(w http.ResponseWriter, r *http.Request) {
	s.scheduleBackup(w, r, service.NewAdHocFullBackupJobForRoutine)
}

// TriggerIncrementalBackup
// @Summary  Trigger an incremental backup once per routine name.
// @ID       triggerIncrementalBackup
// @Tags     Backup
// @Param    name path string true "Backup routine name"
// @Param    delay query int false "Delay interval in milliseconds"
// @Router   /v1/backups/incremental/{name} [post]
// @Success  202
// @Failure  400 {string} string
// @Failure  404 {string} string
// @Failure  500 {string} string
func (s *Service) TriggerIncrementalBackup(w http.ResponseWriter, r *http.Request) {
	s.scheduleBackup(w, r, service.NewAdHocIncrementalBackupJobForRoutine)
}

// ScheduleIncrementalBackup
// @Summary     Schedule an incremental backup once per routine name.
// @Description Deprecated: use POST /v1/backups/incremental/{name} instead.
// @ID          scheduleIncrementalBackup
// @Tags        Backup
// @Deprecated
// @Param       name path string true "Backup routine name"
// @Param       delay query int false "Delay interval in milliseconds"
// @Router      /v1/backups/schedule/incremental/{name} [post]
// @Success     202
// @Failure     400 {string} string
// @Failure     404 {string} string
// @Failure     500 {string} string
func (s *Service) ScheduleIncrementalBackup(w http.ResponseWriter, r *http.Request) {
	s.scheduleBackup(w, r, service.NewAdHocIncrementalBackupJobForRoutine)
}

func (s *Service) scheduleBackup(
	w http.ResponseWriter,
	r *http.Request,
	newJobDetail func(routineName string) *quartz.JobDetail,
) {
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

	jobDetail := newJobDetail(routineName)
	if jobDetail == nil {
		httpError(w, errRoutineNotFound(routineName))
		return
	}

	trigger := quartz.NewRunOnceTrigger(max(time.Duration(delayMillis)*time.Millisecond, minAdHocBackupDelay))

	if err := s.scheduler.ScheduleJob(jobDetail, trigger); err != nil {
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

	routine, found := s.config.Routine(routineName)
	if !found {
		httpError(w, errRoutineNotFound(routineName))
		return
	}

	currentBackups := dto.NewRoutineStateFromModel(s.registry.GetRoutineState(routine))
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

	if _, found := s.config.Routine(routineName); !found {
		httpError(w, errRoutineNotFound(routineName))
		return
	}

	s.registry.Cancel(routineName)

	w.WriteHeader(http.StatusAccepted)
}
