//nolint:dupl
package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/gorilla/mux"
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
	hLogger := s.logger.With(slog.String("handler", "RestoreFullHandler"))

	var request dto.RestoreRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		hLogger.Error("failed to decode request body",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err = request.Validate(); err != nil {
		hLogger.Error("failed to validate request",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jobID, err := s.restoreManager.Restore(request.ToModel())
	if err != nil {
		hLogger.Error("failed to restore",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	hLogger.Info("Restore full",
		slog.Int("jobID", int(jobID)),
		slog.Any("request", request),
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = fmt.Fprint(w, jobID)
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
	hLogger := s.logger.With(slog.String("handler", "RestoreIncrementalHandler"))

	var request dto.RestoreRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		hLogger.Error("failed to decode request body",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err = request.Validate(); err != nil {
		hLogger.Error("failed to validate request",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jobID, err := s.restoreManager.Restore(request.ToModel())
	if err != nil {
		hLogger.Error("failed to restore",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	hLogger.Info("RestoreByPath action",
		slog.Int("jobID", int(jobID)),
		slog.Any("request", request),
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = fmt.Fprint(w, jobID)
}

// RestoreByTimeHandler
// @Summary     Trigger an asynchronous restore operation to specific point in time.
// @ID 	        restoreTimestamp
// @Description Restores backup from the given point in time.
// @Tags        Restore
// @Router      /v1/restore/timestamp [post]
// @Accept      json
// @Param       request body dto.RestoreTimestampRequest true "Restore request details"
// @Success     202 {int64} int64 "Restore operation job id"
// @Failure     400 {string} string
// @Failure     405 {string} string
func (s *Service) RestoreByTimeHandler(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "RestoreByTimeHandler"))

	var request dto.RestoreTimestampRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		hLogger.Error("failed to decode request body",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err = request.Validate(s.config); err != nil {
		hLogger.Error("failed to validate request",
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jobID, err := s.restoreManager.RestoreByTime(request.ToModel())
	if err != nil {
		hLogger.Error("failed to restore by timestamp",
			slog.Any("routine", request.Routine),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	hLogger.Info("Restore action",
		slog.Int("jobID", int(jobID)),
		slog.Any("request", request),
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = fmt.Fprint(w, jobID)
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
func (s *Service) RestoreStatusHandler(w http.ResponseWriter, r *http.Request) {
	hLogger := s.logger.With(slog.String("handler", "RestoreStatusHandler"))

	jobIDParam := mux.Vars(r)["jobId"]

	if jobIDParam == "" {
		hLogger.Error("job id required")
		http.Error(w, "jobId required", http.StatusBadRequest)
		return
	}
	jobID, err := strconv.Atoi(jobIDParam)
	if err != nil {
		hLogger.Error("failed to parse job id",
			slog.String("jobIDParam", jobIDParam),
			slog.Any("error", err))
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	status, err := s.restoreManager.JobStatus(model.RestoreJobID(jobID))
	if err != nil {
		hLogger.Error("failed to get job status",
			slog.Int("jobID", jobID),
			slog.Any("error", err),
		)
		http.Error(w, err.Error(), http.StatusNotFound)
	}
	jsonResponse, err := dto.Serialize(dto.NewResultFromModel(status), dto.JSON)
	w.WriteHeader(http.StatusOK)
	if err != nil {
		hLogger.Error("failed to marshal restore result",
			slog.Any("error", err),
		)
		http.Error(w, "failed to parse restore status", http.StatusInternalServerError)
		return
	}
	_, err = w.Write(jsonResponse)
	if err != nil {
		hLogger.Error("failed to write response",
			slog.String("response", string(jsonResponse)),
			slog.Any("error", err),
		)
	}
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
	hLogger := s.logger.With(slog.String("handler", "RetrieveConfig"))

	name := mux.Vars(r)["name"]
	if name == "" {
		hLogger.Error("routine name required")
		http.Error(w, "Routine name required", http.StatusBadRequest)
		return
	}
	timestampStr := mux.Vars(r)["timestamp"]
	if timestampStr == "" {
		hLogger.Error("timestamp required")
		http.Error(w, "Timestamp required", http.StatusBadRequest)
		return
	}

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		hLogger.Error("failed to parse timestamp",
			slog.String("timestamp", timestampStr),
			slog.Any("error", err))
		http.Error(w, "Timestamp incorrect", http.StatusBadRequest)
		return
	}

	buf, err := s.restoreManager.RetrieveConfiguration(name, time.UnixMilli(timestamp))
	if err != nil {
		hLogger.Error("failed to retrieve config",
			slog.Int64("timestamp", timestamp),
			slog.String("name", name),
			slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="archive.zip"`)
	_, err = w.Write(buf)
	if err != nil {
		hLogger.Error("failed to write response",
			slog.String("response", string(buf)),
			slog.Any("error", err),
		)
	}
}
