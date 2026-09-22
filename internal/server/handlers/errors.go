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

// badRequestError marks an error as the client's fault: the request was understood but cannot be
// accepted. It carries no status of its own — every other code comes from a model.EntityError
// cause or from one of httpError's own cases, so there is a single place each status is decided.
type badRequestError struct {
	Err error
}

func (e *badRequestError) Error() string {
	return e.Err.Error()
}

// Unwrap exposes the cause, so errors.Is/As can see through the wrapper.
func (e *badRequestError) Unwrap() error {
	return e.Err
}

func newBadRequestError(err error) *badRequestError {
	return &badRequestError{Err: err}
}

func errInvalidQueryParam(err error, param string) error {
	return newBadRequestError(fmt.Errorf("invalid query param %s: %w", param, err))
}

func errBadRequest(err error) error {
	return newBadRequestError(fmt.Errorf("invalid request: %w", err))
}

var errMissingRoutineName = newBadRequestError(errors.New("routine name required"))
var errMissingClusterName = newBadRequestError(errors.New("cluster name required"))
var errMissingPolicyName = newBadRequestError(errors.New("policy name required"))
var errMissingStorageName = newBadRequestError(errors.New("storage name required"))

// httpError is the one place an error becomes a status code. The entity causes are matched before
// the bad-request wrapper, so wrapping one can never hide it the way it did before BKRS-439; a cause with
// no case here is a fault in the service, not in the request, and reads as 500.
func httpError(w http.ResponseWriter, err error) {
	var (
		badRequest  *badRequestError
		maxBytesErr *http.MaxBytesError
	)

	switch {
	case errors.As(err, &maxBytesErr):
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
	case errors.Is(err, model.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, model.ErrAlreadyExists), errors.Is(err, model.ErrInUse):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.As(err, &badRequest):
		http.Error(w, badRequest.Error(), http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// writeRedactedJSON marshals v to JSON with secret fields redacted and writes it to w.
func writeRedactedJSON(w http.ResponseWriter, v any) {
	body, _ := decoder.Marshal(v, decoder.JSON, true)

	// #nosec G705 -- body is JSON from decoder.Marshal with secret redaction, not reflected HTML
	_, _ = w.Write(body)
}

// httpOK responds with a JSON-encoded success message and 200 status.
// Secret fields are redacted in the response body.
func httpOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	writeRedactedJSON(w, data)
}

// httpAcceptedWithJobID responds with a job ID and 202 status.
func httpAcceptedWithJobID(w http.ResponseWriter, jobID model.RestoreJobID) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	// #nosec G705 -- jobID is a numeric identifier issued by the restore manager, written as %d
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
