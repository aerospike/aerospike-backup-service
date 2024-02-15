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
// @Description Specify the directory parameter for the full backup restore.
// @Tags        Restore
// @Router      /v1/restore/full [post]
// @Accept      json
// @Param       request body model.RestoreRequest true "query params"
// @Success     202 {int64}  "Job ID"
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
		fmt.Fprint(w, jobID)
	} else {
		http.Error(w, "", http.StatusNotFound)
	}
}

// @Summary     Trigger an asynchronous incremental restore operation.
// @ID 	        restoreIncremental
// @Description Specify the file parameter to restore from an incremental backup file.
// @Tags        Restore
// @Router      /v1/restore/incremental [post]
// @Accept      json
// @Param       request body model.RestoreRequest true "query params"
// @Success     202 {int64}  "Job ID"
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
		fmt.Fprint(w, jobID)
	} else {
		http.Error(w, "", http.StatusNotFound)
	}
}

// @Summary     Trigger an asynchronous restore operation to specific point in time.
// @ID 	        restoreTimestamp
// @Description Restores backup from given point in time
// @Tags        Restore
// @Router      /v1/restore/timestamp [post]
// @Accept      json
// @Param       request body model.RestoreTimestampRequest true "query params"
// @Success     202 {int64}  "Job ID"
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
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slog.Info("Restore action", "jobID", jobID, "request", request)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, jobID)
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
// @Success     200 {object} model.RestoreJobStatus "Job status"
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
