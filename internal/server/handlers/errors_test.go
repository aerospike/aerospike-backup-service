package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
)

// httpError is the only place a status is chosen, and which case wins is decided by their order:
// an entity cause is matched before the bad-request wrapper, so wrapping one cannot hide it. That
// ordering is the whole of the BKRS-439 fix — before it, errors.As on the wrapper ran first and
// made 404 unreachable from every config handler — so it is pinned here rather than left to the
// end-to-end rows in TestUnconfiguredService_APIContract.
func TestHTTPErrorStatus(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "a missing entity reads as not found",
			err:        model.NotFound("routine", "daily"),
			wantStatus: http.StatusNotFound,
			wantBody:   `routine "daily" not found`,
		},
		{
			name:       "a taken name conflicts",
			err:        model.AlreadyExists("storage", "s3-backup"),
			wantStatus: http.StatusConflict,
			wantBody:   `storage "s3-backup" already exists`,
		},
		{
			name:       "a referenced entity conflicts and names the holder",
			err:        model.InUse("policy", "keep-7", `it is used in routine "daily"`),
			wantStatus: http.StatusConflict,
			wantBody:   `policy "keep-7" is in use: it is used in routine "daily"`,
		},
		{
			name:       "a cause wrapped in the bad-request wrapper keeps its own status",
			err:        errBadRequest(model.NotFound("routine", "daily")),
			wantStatus: http.StatusNotFound,
			wantBody:   `routine "daily" not found`,
		},
		{
			name:       "a conflict wrapped in the bad-request wrapper keeps its own status",
			err:        errBadRequest(model.AlreadyExists("storage", "s3-backup")),
			wantStatus: http.StatusConflict,
		},
		{
			name:       "a cause several layers down is still found",
			err:        fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", model.NotFound("cluster", "c1"))),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "a request the service cannot accept is a bad request",
			err:        errBadRequest(errors.New("seed nodes are not specified")),
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid request: seed nodes are not specified",
		},
		{
			name:       "a missing path value is a bad request",
			err:        errMissingRoutineName,
			wantStatus: http.StatusBadRequest,
			wantBody:   "routine name required",
		},
		{
			name:       "an oversized body outranks every other case",
			err:        errBadRequest(&http.MaxBytesError{Limit: 10}),
			wantStatus: http.StatusRequestEntityTooLarge,
			wantBody:   "request body too large",
		},
		{
			name:       "an unmapped error is the service's fault, not the request's",
			err:        errors.New("failed to write configuration: disk full"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "failed to write configuration: disk full",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			httpError(w, tt.err)

			assert.Equal(t, tt.wantStatus, w.Code, w.Body.String())
			if tt.wantBody != "" {
				assert.Contains(t, w.Body.String(), tt.wantBody)
			}
		})
	}
}
