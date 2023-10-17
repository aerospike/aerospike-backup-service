package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"log/slog"
)

// @Summary  Get available full backups.
// @Tags     Backup
// @Produce  plain
// @Param    name query string true "Backup policy name"
// @Router   /backup/full/list [get]
// @Success  200 {array} model.BackupDetails "Full backups"
// @Failure  404 {string} string ""
func (ws *HTTPServer) getAvailableFullBackups(w http.ResponseWriter, r *http.Request) {
	policyName := r.URL.Query().Get("name")
	if policyName == "" {
		http.Error(w, "Invalid/undefined policy name", http.StatusBadRequest)
	} else {
		list, err := ws.backupBackends[policyName].FullBackupList()
		if err != nil {
			slog.Error("Get full backup list", "err", err)
			http.Error(w, "", http.StatusNotFound)
		} else {
			response, err := json.Marshal(list)
			if err != nil {
				slog.Error("Failed to parse full backup list", "err", err)
				http.Error(w, "", http.StatusInternalServerError)
			} else {
				fmt.Fprint(w, string(response))
			}
		}
	}
}

// @Summary  Get available incremental backups.
// @Tags     Backup
// @Produce  plain
// @Param    name query string true "Backup policy name"
// @Router   /backup/incremental/list [get]
// @Success  200 {array} model.BackupDetails "Incremental backups"
// @Failure  404 {string} string ""
func (ws *HTTPServer) getAvailableIncrBackups(w http.ResponseWriter, r *http.Request) {
	policyName := r.URL.Query().Get("name")
	if policyName == "" {
		http.Error(w, "Invalid/undefined policy name", http.StatusBadRequest)
	} else {
		list, err := ws.backupBackends[policyName].IncrementalBackupList()
		if err != nil {
			slog.Error("Get incremental backup list", "err", err)
			http.Error(w, "", http.StatusNotFound)
		} else {
			response, err := json.Marshal(list)
			if err != nil {
				slog.Error("Failed to parse incremental backup list", "err", err)
				http.Error(w, "", http.StatusInternalServerError)
			} else {
				fmt.Fprint(w, string(response))
			}
		}
	}
}
