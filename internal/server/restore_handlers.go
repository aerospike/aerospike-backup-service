package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"log/slog"

	"github.com/aerospike/backup/pkg/model"
)

// @Summary     Trigger an asynchronous restore operation.
// @ID 	        restore
// @Description Specify the directory parameter for the full backup restore.
// @Description Use the file parameter to restore from an incremental backup file.
// @Tags        Restore
// @Router      /restore [post]
// @Accept		json
// @Param       request body model.RestoreRequest true "query params"
// @Success     202 {string}  "Job ID (int64)"
// @Failure     400 {string} string
func (ws *HTTPServer) restoreHandler(w http.ResponseWriter, r *http.Request) {
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
		jobID := ws.restoreService.Restore(&request)
		slog.Info("Restore action", "jobID", jobID, "request", request)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, strconv.Itoa(jobID))
	} else {
		http.Error(w, "", http.StatusNotFound)
	}
}

// @Summary     Trigger an asynchronous restore operation.
// @ID 	        restoreTime
// @Description Restores backup from given point in time
// @Tags        Restore
// @Router      /restoreTime [post]
// @Accept		json
// @Param       request body model.RestoreTimeRequest true "query params"
// @Success     202 {string}  "Job ID (int64)"
// @Failure     400 {string} string
func (ws *HTTPServer) restoreByTimeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var request model.RestoreTimeRequest

		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err = request.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		jobID := ws.restoreService.RestoreByTime(&request)
		slog.Info("Restore action", "jobID", jobID, "request", request)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, strconv.Itoa(jobID))
	} else {
		http.Error(w, "", http.StatusNotFound)
	}
}

// @Summary     Retrieve status for a restore job.
// @ID	        restoreStatus
// @Tags        Restore
// @Produce     json
// @Param       jobId query string true "Job ID to retrieve the status"
// @Router      /restore/status [get]
// @Success     200 {string} string "Job status"
// @Failure     400 {string} string
func (ws *HTTPServer) restoreStatusHandler(w http.ResponseWriter, r *http.Request) {
	jobIDParam := r.URL.Query().Get("jobId")
	jobID, err := strconv.Atoi(jobIDParam)
	if err != nil {
		http.Error(w, "Invalid job id", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, ws.restoreService.JobStatus(jobID))
}
