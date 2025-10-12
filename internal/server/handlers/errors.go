package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// errorWithCode represents an error with an associated HTTP status code.
type errorWithCode struct {
	Err  error
	Code int
}

func (e *errorWithCode) Error() string {
	return e.Err.Error()
}

// newErrorWithCode creates a new errorWithCode with an associated status code.
func newErrorWithCode(err error, code int) *errorWithCode {
	return &errorWithCode{Err: err, Code: code}
}

func errInvalidQueryParam(err error, param string) error {
	return newErrorWithCode(fmt.Errorf("invalid query param %s: %w", param, err), http.StatusBadRequest)
}

func errInvalidJSONPayload(err error) error {
	return newErrorWithCode(fmt.Errorf("invalid JSON payload: %w", err), http.StatusBadRequest)
}

func errBadRequest(err error) error {
	return newErrorWithCode(fmt.Errorf("invalid request: %w", err), http.StatusBadRequest)
}

var errMissingRoutineName = newErrorWithCode(errors.New("routine name required"), http.StatusBadRequest)
var errMissingClusterName = newErrorWithCode(errors.New("cluster name required"), http.StatusBadRequest)
var errMissingPolicyName = newErrorWithCode(errors.New("policy name required"), http.StatusBadRequest)
var errMissingStorageName = newErrorWithCode(errors.New("storage name required"), http.StatusBadRequest)

func errRoutineNotFound(name string) error {
	return errNotFound("routine", name)
}

func errNotFound(field string, name any) error {
	return newErrorWithCode(fmt.Errorf("%s %q not found", field, name), http.StatusNotFound)
}

// httpError calls http.Error with the appropriate status code based on the error.
func httpError(w http.ResponseWriter, err error) {
	var httpErr *errorWithCode

	switch {
	case errors.As(err, &httpErr):
		http.Error(w, httpErr.Error(), httpErr.Code)
	case errors.Is(err, model.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// httpOK responds with a JSON-encoded success message and 200 status.
func httpOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Error encoding JSON response", attr.Error(err))
	}
}

// httpAcceptedWithJobID responds with a job ID and 202 status.
func httpAcceptedWithJobID(w http.ResponseWriter, jobID model.RestoreJobID) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, err := fmt.Fprintf(w, "%d", jobID)
	if err != nil {
		slog.Error("Error encoding job id response", attr.Error(err))
	}
}

// httpAccepted responds with an empty 202 Accepted.
func httpAccepted(w http.ResponseWriter) {
	w.WriteHeader(http.StatusAccepted)
}

// httpContent sends a file as an HTTP response with a dynamically determined content type.
func httpContent(w http.ResponseWriter, buf []byte, filename string) {
	contentType := mime.TypeByExtension(filepath.Ext(filename))

	// Fallback for unknown MIME types
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(buf)))

	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(buf); err != nil {
		slog.Error("Error encoding writing response", attr.Error(err))
		http.Error(w, "Failed to send file", http.StatusInternalServerError)
	}
}
