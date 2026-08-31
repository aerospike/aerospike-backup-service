package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/stretchr/testify/assert"
)

func newTestServeMux(t *testing.T) *http.ServeMux {
	t.Helper()

	svc := handlers.NewService(
		t.Context(),
		model.NewConfig(),
		nil, nil, nil, nil, nil, nil, nil, nil,
		secrets.NewResolver(),
	)

	return NewServeMux("/v1", "/", svc)
}

// TestNewServeMux_RoutesRegistered asserts that every route wired in NewServeMux is served by its
// own pattern, without invoking the handler logic itself.
//
// The expected pattern must be asserted rather than just "some handler was found": "GET /" is a
// catch-all, so every unmatched GET request still resolves to a handler. Comparing patterns is
// what makes a removed or renamed GET route fail here instead of silently falling back to root.
func TestNewServeMux_RoutesRegistered(t *testing.T) {
	mux := newTestServeMux(t)

	tests := []struct {
		method      string
		path        string
		wantPattern string
	}{
		{http.MethodGet, "/", "GET /"},
		{http.MethodGet, "/health", "GET /health"},
		{http.MethodGet, "/ready", "GET /ready"},
		{http.MethodGet, "/version", "GET /version"},
		{http.MethodGet, "/metrics", "GET /metrics"},
		{http.MethodGet, "/api-docs/", "GET /api-docs/"},

		{http.MethodGet, "/v1/config", "GET /v1/config"},
		{http.MethodPut, "/v1/config", "PUT /v1/config"},
		{http.MethodPost, "/v1/config/apply", "POST /v1/config/apply"},

		{http.MethodGet, "/v1/config/clusters", "GET /v1/config/clusters"},
		{http.MethodPost, "/v1/config/clusters/cluster1", "POST /v1/config/clusters/{name}"},
		{http.MethodGet, "/v1/config/clusters/cluster1", "GET /v1/config/clusters/{name}"},
		{http.MethodPut, "/v1/config/clusters/cluster1", "PUT /v1/config/clusters/{name}"},
		{http.MethodDelete, "/v1/config/clusters/cluster1", "DELETE /v1/config/clusters/{name}"},

		{http.MethodGet, "/v1/config/storage", "GET /v1/config/storage"},
		{http.MethodPost, "/v1/config/storage/storage1", "POST /v1/config/storage/{name}"},
		{http.MethodGet, "/v1/config/storage/storage1", "GET /v1/config/storage/{name}"},
		{http.MethodPut, "/v1/config/storage/storage1", "PUT /v1/config/storage/{name}"},
		{http.MethodDelete, "/v1/config/storage/storage1", "DELETE /v1/config/storage/{name}"},

		{http.MethodGet, "/v1/config/policies", "GET /v1/config/policies"},
		{http.MethodPost, "/v1/config/policies/policy1", "POST /v1/config/policies/{name}"},
		{http.MethodGet, "/v1/config/policies/policy1", "GET /v1/config/policies/{name}"},
		{http.MethodPut, "/v1/config/policies/policy1", "PUT /v1/config/policies/{name}"},
		{http.MethodDelete, "/v1/config/policies/policy1", "DELETE /v1/config/policies/{name}"},

		{http.MethodGet, "/v1/config/routines", "GET /v1/config/routines"},
		{http.MethodPost, "/v1/config/routines/routine1", "POST /v1/config/routines/{name}"},
		{http.MethodGet, "/v1/config/routines/routine1", "GET /v1/config/routines/{name}"},
		{http.MethodPut, "/v1/config/routines/routine1", "PUT /v1/config/routines/{name}"},
		{http.MethodDelete, "/v1/config/routines/routine1", "DELETE /v1/config/routines/{name}"},
		{http.MethodPut, "/v1/config/routines/routine1/disable", "PUT /v1/config/routines/{name}/disable"},
		{http.MethodPut, "/v1/config/routines/routine1/enable", "PUT /v1/config/routines/{name}/enable"},

		{http.MethodGet, "/v1/backups/full", "GET /v1/backups/full"},
		{http.MethodGet, "/v1/backups/full/routine1", "GET /v1/backups/full/{name}"},
		{http.MethodGet, "/v1/backups/incremental", "GET /v1/backups/incremental"},
		{http.MethodGet, "/v1/backups/incremental/routine1", "GET /v1/backups/incremental/{name}"},
		{http.MethodPost, "/v1/backups/full/routine1", "POST /v1/backups/full/{name}"},
		{http.MethodPost, "/v1/backups/incremental/routine1", "POST /v1/backups/incremental/{name}"},
		{http.MethodPost, "/v1/backups/schedule/routine1", "POST /v1/backups/schedule/{name}"},
		{http.MethodGet, "/v1/backups/currentBackup/routine1", "GET /v1/backups/currentBackup/{name}"},
		{http.MethodPost, "/v1/backups/cancel/routine1", "POST /v1/backups/cancel/{name}"},

		{http.MethodPost, "/v1/restore/full", "POST /v1/restore/full"},
		{http.MethodPost, "/v1/restore/incremental", "POST /v1/restore/incremental"},
		{http.MethodPost, "/v1/restore/timestamp", "POST /v1/restore/timestamp"},
		{http.MethodGet, "/v1/restore/status/1", "GET /v1/restore/status/{jobId}"},
		{http.MethodGet, "/v1/restore/jobs", "GET /v1/restore/jobs"},
		{http.MethodPost, "/v1/restore/cancel/1", "POST /v1/restore/cancel/{jobId}"},
		{http.MethodGet, "/v1/retrieve/configuration/routine1/1000", "GET /v1/retrieve/configuration/{name}/{timestamp}"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)

			_, pattern := mux.Handler(req)

			assert.Equal(t, tt.wantPattern, pattern,
				"%s %s should be served by its own route", tt.method, tt.path)
		})
	}
}

// TestNewServeMux_UnknownPathFallsBackToRoot documents why TestNewServeMux_RoutesRegistered has to
// compare patterns: an unknown GET path is still matched by the "GET /" catch-all, and only
// RootActionHandler turns it into a 404.
func TestNewServeMux_UnknownPathFallsBackToRoot(t *testing.T) {
	mux := newTestServeMux(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/no-such-route", nil)

	_, pattern := mux.Handler(req)
	assert.Equal(t, "GET /", pattern)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestNewServeMux_MethodNotAllowed verifies that a registered path rejects
// an unsupported HTTP method (the "/" catch-all registered for the root path
// means unmatched paths fall through there, so we assert on the method mismatch
// for a path with no fallback instead of a literal "not found" pattern).
func TestNewServeMux_MethodNotAllowed(t *testing.T) {
	mux := newTestServeMux(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/v1/config/clusters/cluster1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}
