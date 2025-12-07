package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
)

// LogsActionHandler
// @Summary     Returns system logs.
// @ID	        logs
// @Tags        System
// @Router      /logs [get]
// @Success 	200 {array} log.LogEntry
func (s *Service) LogsActionHandler(w http.ResponseWriter, _ *http.Request) {
	if s.logCaptureHandler == nil {
		http.Error(w, "Log capture is not enabled", http.StatusServiceUnavailable)
		return
	}

	logs := s.logCaptureHandler.GetEntries()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(logs); err != nil {
		slog.Error("failed to write response", attr.Error(err))
		http.Error(w, "Failed to encode logs", http.StatusInternalServerError)
	}
}
