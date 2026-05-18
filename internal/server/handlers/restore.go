package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

// RestoreFullHandler
// @Summary     Trigger an asynchronous full restore operation.
// @ID 	        restoreFull
// @Tags        Restore
// @Router      /v1/restore/full [post]
// @Accept      json
// @Param       request body dto.RestoreRequest true "Restore request details"
// @Success     202 {int64} int64 "Restore operation job id"
// @Failure     400 {string} string
// @Failure     405 {string} string
func (s *Service) RestoreFullHandler(w http.ResponseWriter, r *http.Request) {
	s.restoreByPath(w, r)
}

// RestoreIncrementalHandler
// @Summary     Trigger an asynchronous incremental restore operation.
// @ID 	        restoreIncremental
// @Tags        Restore
// @Router      /v1/restore/incremental [post]
// @Accept      json
// @Param       request body dto.RestoreRequest true "Restore request details"
// @Success     202 {int64} int64 "Restore operation job id"
// @Failure     400 {string} string
// @Failure     405 {string} string
func (s *Service) RestoreIncrementalHandler(w http.ResponseWriter, r *http.Request) {
	s.restoreByPath(w, r)
}

// RestoreIncremental and RestoreFull share same business logic.
func (s *Service) restoreByPath(w http.ResponseWriter, r *http.Request) {
	request, err := dto.NewRestoreRequestFromReader(r.Body)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}
	if err = request.Validate(); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	restoreRequest, err := request.ToModel(s.config)
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	jobID, err := s.restoreManager.Restore(s.sysCtx, restoreRequest)
	if err != nil {
		httpError(w, err)
		return
	}

	httpAcceptedWithJobID(w, jobID)
}

// RestoreByTimeHandler
// @Summary     Trigger an asynchronous restore operation to specific point in time.
// @ID 	        restoreTimestamp
// @Description Restore DB to a specific point in time by applying the latest backup preceding that time.
// @Tags        Restore
// @Router      /v1/restore/timestamp [post]
// @Accept      json
// @Param       request body dto.RestoreTimestampRequest true "Restore request details"
// @Success     202 {int64} int64 "Restore operation job id"
// @Failure     400 {string} string
// @Failure     405 {string} string
func (s *Service) RestoreByTimeHandler(w http.ResponseWriter, r *http.Request) {
	request, err := dto.NewRestoreTimestampRequestFromReader(r.Body)

	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return
	}
	if err = request.Validate(); err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	restoreRequest, err := request.ToModel(s.config)
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	jobID, err := s.restoreManager.RestoreByTime(s.sysCtx, restoreRequest)
	if err != nil {
		httpError(w, errBadRequest(err))
		return
	}

	httpAcceptedWithJobID(w, jobID)
}

// RestoreStatusHandler
// @Summary     Retrieve status for a restore job.
// @ID	        restoreStatus
// @Tags        Restore
// @Produce     json
// @Param       jobId path int true "Job ID to retrieve the status" format(int64)
// @Router      /v1/restore/status/{jobId} [get]
// @Success     200 {object} dto.RestoreJobStatus "Restore job status details"
// @Failure     400 {string} string
// @Failure     404 {string} string "The specified restore job was not found"
func (s *Service) RestoreStatusHandler(w http.ResponseWriter, r *http.Request) {
	jobID, err := extractJobID(r)
	if err != nil {
		httpError(w, errInvalidQueryParam(err, "jobId"))
		return
	}

	status, err := s.restoreManager.JobStatus(jobID)
	if err != nil {
		var jobErr *service.ErrJobNotFound
		if errors.As(err, &jobErr) {
			httpError(w, errNotFound("job", jobID))
		} else {
			httpError(w, err)
		}
		return
	}

	httpOK(w, dto.NewResultFromModel(status))
}

func extractJobID(r *http.Request) (model.RestoreJobID, error) {
	jobIDParam := r.PathValue("jobId")
	if jobIDParam == "" {
		return 0, errors.New("jobId required")
	}
	jobID, err := strconv.ParseInt(jobIDParam, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid jobId %q", jobIDParam)
	}
	return model.RestoreJobID(jobID), nil
}

// RetrieveRestoreJobs
// @Summary  Retrieve restore jobs.
// @ID       retrieveRestoreJobs
// @Tags     Restore
// @Produce  json
// @Param    from query int false "Lower bound timestamp filter" format(int64)
// @Param    to query int false "Upper bound timestamp filter" format(int64)
// @Param    status query string false "Comma-separated filter: running, success, failure, canceled (case-insensitive). Aliases done→success, failed→failure. Prefix ! excludes (e.g. !canceled)"
// @Router   /v1/restore/jobs [get]
// @Success  200 {object} map[string]dto.RestoreJobStatus "Restore jobs"
// @Failure  400 {string} string
//
//nolint:lll
func (s *Service) RetrieveRestoreJobs(w http.ResponseWriter, r *http.Request) {
	timeBounds, err := dto.NewTimeBoundsFromString(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		httpError(w, errInvalidQueryParam(err, "time bounds"))
		return
	}

	statusFilter, err := dto.NewStatusFilterFromString(r.URL.Query().Get("status"))
	if err != nil {
		httpError(w, errInvalidQueryParam(err, "status"))
		return
	}

	jobs := s.restoreManager.GetFilteredJobs(timeBounds, statusFilter)
	result := make(map[string]*dto.RestoreJobStatus, len(jobs))
	for key, m := range jobs {
		result[strconv.FormatInt(int64(key), 10)] = dto.NewResultFromModel(m)
	}

	httpOK(w, result)
}

// RetrieveConfig
// @Summary     Retrieve Aerospike cluster configuration backup
// @ID	        retrieveConfiguration
// @Tags        Restore
// @Produce     application/zip
// @Param       name path string true "Backup routine name"
// @Param       timestamp path int true "Backup timestamp" format(int64)
// @Router      /v1/retrieve/configuration/{name}/{timestamp} [get]
// @Success     200 {file} application/zip "configuration backup"
// @Failure     400 {string} string
// @Failure     405 {string} string
func (s *Service) RetrieveConfig(w http.ResponseWriter, r *http.Request) {
	routineName := r.PathValue("name")
	if routineName == "" {
		httpError(w, errMissingRoutineName)
		return
	}
	timestampStr := r.PathValue("timestamp")
	if timestampStr == "" {
		httpError(w, errMissingStorageName)
		return
	}

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		httpError(w, errInvalidQueryParam(err, "timestamp"))
		return
	}

	routine, found := s.config.Routine(routineName)
	if !found {
		httpError(w, errRoutineNotFound(routineName))
		return
	}

	buf, err := s.configRetriever.RetrieveConfiguration(r.Context(), routine, time.UnixMilli(timestamp))
	if err != nil {
		httpError(w, err)
		return
	}

	httpContent(w, buf, "archive.zip")
}

// CancelRestoreHandler
// @Summary     Cancel a running restore operation.
// @ID          cancelRestore
// @Tags        Restore
// @Router      /v1/restore/cancel/{jobId} [post]
// @Param       jobId path int true "Restore job ID" format(int64)
// @Success     202 {string} string "Restore job canceled successfully"
// @Failure     400 {string} string "Invalid job ID"
// @Failure     404 {string} string "The specified restore job was not found"
func (s *Service) CancelRestoreHandler(w http.ResponseWriter, r *http.Request) {
	jobID, err := extractJobID(r)
	if err != nil {
		httpError(w, errInvalidQueryParam(err, "jobId"))
		return
	}

	err = s.restoreManager.CancelRestore(jobID)
	if err != nil {
		var jobErr *service.ErrJobNotFound
		if errors.As(err, &jobErr) {
			httpError(w, errNotFound("job", jobID))
		} else {
			httpError(w, fmt.Errorf("failed to cancel restore: %w", err))
		}
		return
	}

	httpAccepted(w)
}
