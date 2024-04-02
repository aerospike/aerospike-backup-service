//nolint:dupl
package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/aerospike/backup/pkg/model"
)

// @Summary     Trigger an asynchronous full restore operation.
// @ID 	        restoreFull
// @Tags        Restore
// @Router      /v1/restore/full [post]
// @Accept      json
// @Param       request body model.RestoreRequest true "Restore request details"
// @Success     202 {int64} int64 "Restore operation job id"
// @Failure     400 {string} string
func (ws *HTTPServer) restoreFullHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var request model.RestoreRequest

		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err = request.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requestInternal := &model.RestoreRequestInternal{
			RestoreRequest: request,
			Dir:            request.SourceStorage.Path,
		}

		jobID, err := ws.restoreService.Restore(requestInternal)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slog.Info("Restore full", "jobID", jobID, "request", request)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = fmt.Fprint(w, jobID)
	} else {
		http.Error(w, "", http.StatusNotFound)
	}
}

// @Summary     Trigger an asynchronous incremental restore operation.
// @ID 	        restoreIncremental
// @Tags        Restore
// @Router      /v1/restore/incremental [post]
// @Accept      json
// @Param       request body model.RestoreRequest true "Restore request details"
// @Success     202 {int64} int64 "Restore operation job id"
// @Failure     400 {string} string
func (ws *HTTPServer) restoreIncrementalHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var request model.RestoreRequest

		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err = request.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requestInternal := &model.RestoreRequestInternal{
			RestoreRequest: request,
			Dir:            request.SourceStorage.Path,
		}
		jobID, err := ws.restoreService.Restore(requestInternal)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slog.Info("RestoreByPath action", "jobID", jobID, "request", request)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = fmt.Fprint(w, jobID)
	} else {
		http.Error(w, "", http.StatusNotFound)
	}
}

// @Summary     Trigger an asynchronous restore operation to specific point in time.
// @ID 	        restoreTimestamp
// @Description Restores backup from the given point in time.
// @Tags        Restore
// @Router      /v1/restore/timestamp [post]
// @Accept      json
// @Param       request body model.RestoreTimestampRequest true "Restore request details"
// @Success     202 {int64} int64 "Restore operation job id"
// @Failure     400 {string} string
func (ws *HTTPServer) restoreByTimeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var request model.RestoreTimestampRequest

		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err = request.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		jobID, err := ws.restoreService.RestoreByTime(&request)
		if err != nil {
			slog.Error("Restore by timestamp failed", "routine", request.Routine, "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slog.Info("Restore action", "jobID", jobID, "request", request)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = fmt.Fprint(w, jobID)
	} else {
		http.Error(w, "", http.StatusNotFound)
	}
}

// @Summary     Retrieve status for a restore job.
// @ID	        restoreStatus
// @Tags        Restore
// @Produce     json
// @Param       jobId path int true "Job ID to retrieve the status" format(int64)
// @Router      /v1/restore/status/{jobId} [get]
// @Success     200 {object} model.RestoreJobStatus "Restore job status details"
// @Failure     400 {string} string
func (ws *HTTPServer) restoreStatusHandler(w http.ResponseWriter, r *http.Request) {
	jobIDParam := r.PathValue("jobId")
	if jobIDParam == "" {
		http.Error(w, "jobId required", http.StatusBadRequest)
		return
	}
	jobID, err := strconv.Atoi(jobIDParam)
	if err != nil {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	status, err := ws.restoreService.JobStatus(jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	} else {
		w.WriteHeader(http.StatusOK)
		jsonResponse, err := json.MarshalIndent(status, "", "    ") // pretty print
		if err != nil {
			http.Error(w, "failed to parse restore status", http.StatusInternalServerError)
			return
		}
		_, err = w.Write(jsonResponse)
		if err != nil {
			slog.Error("failed to write response", "err", err)
		}
	}
}

// @Summary     Retrieve Aerospike cluster configuration backup
// @ID	        retrieveConfiguration
// @Tags        Restore
// @Produce     application/zip
// @Param       name path string true "Backup routine name"
// @Param       timestamp path int true "Backup timestamp" format(int64)
// @Router      /v1/retrieve/configuration/{name}/{timestamp} [get]
// @Success     200 {file} application/zip "configuration backup"
// @Failure     400 {string} string
func (ws *HTTPServer) retrieveConfig(w http.ResponseWriter, r *http.Request) {
	// Check if method is GET
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "Routine name required", http.StatusBadRequest)
		return
	}
	timestampStr := r.PathValue("timestamp")
	if timestampStr == "" {
		http.Error(w, "Timestamp required", http.StatusBadRequest)
		return
	}

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		http.Error(w, "Timestamp incorrect", http.StatusBadRequest)
		return
	}

	buf, err := ws.restoreService.RetrieveConfiguration(name, timestamp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="archive.zip"`)
	_, err = w.Write(buf)
	if err != nil {
		slog.Error("failed to write response", "err", err)
	}
}
