package handlers

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// statusCodeError represents an error with an associated HTTP status code.
type statusCodeError struct {
	Err  error
	Code int
}

func (e *statusCodeError) Error() string {
	return e.Err.Error()
}

// newStatusCodeError creates a new statusCodeError with an associated status code.
func newStatusCodeError(err error, code int) *statusCodeError {
	return &statusCodeError{Err: err, Code: code}
}

func errInvalidQueryParam(err error, param string) error {
	return newStatusCodeError(fmt.Errorf("invalid query param %s: %w", param, err), http.StatusBadRequest)
}

func errInvalidJSONPayload(err error) error {
	return newStatusCodeError(fmt.Errorf("invalid JSON payload: %w", err), http.StatusBadRequest)
}

func errBadRequest(err error) error {
	return newStatusCodeError(fmt.Errorf("invalid request: %w", err), http.StatusBadRequest)
}

var errMissingRoutineName = newStatusCodeError(errors.New("routine name required"), http.StatusBadRequest)
var errMissingClusterName = newStatusCodeError(errors.New("cluster name required"), http.StatusBadRequest)
var errMissingPolicyName = newStatusCodeError(errors.New("policy name required"), http.StatusBadRequest)
var errMissingStorageName = newStatusCodeError(errors.New("storage name required"), http.StatusBadRequest)

func errRoutineNotFound(name string) error {
	return errNotFound("routine", name)
}

func errNotFound(field string, name any) error {
	return newStatusCodeError(fmt.Errorf("%s %q not found", field, name), http.StatusNotFound)
}

// httpError calls http.Error with the appropriate status code based on the error.
func httpError(w http.ResponseWriter, err error) {
	var httpErr *statusCodeError

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

	_ = decoder.Serialize(w, data, decoder.JSON, true)
}

// httpAcceptedWithJobID responds with a job ID and 202 status.
func httpAcceptedWithJobID(w http.ResponseWriter, jobID model.RestoreJobID) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = fmt.Fprintf(w, "%d", jobID)
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
	w.Header().Set("Content-Length", strconv.Itoa(len(buf)))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	w.WriteHeader(http.StatusOK)

	_, _ = w.Write(buf)
}
