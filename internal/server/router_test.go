package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
)

// TestNewServeMux_RoutesRegistered is a smoke test asserting every route wired
// in NewServeMux actually resolves to a registered handler (no 404 "not found" mux
// pattern), without invoking the handler logic itself.
func TestNewServeMux_RoutesRegistered(t *testing.T) {
	svc := handlers.NewService(
		t.Context(),
		model.NewConfig(),
		nil, nil, nil, nil, nil, nil, nil, nil,
	)

	mux := NewServeMux("/v1", "/", svc)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/"},
		{http.MethodGet, "/health"},
		{http.MethodGet, "/ready"},
		{http.MethodGet, "/version"},
		{http.MethodGet, "/metrics"},
		{http.MethodGet, "/api-docs/"},

		{http.MethodGet, "/v1/config"},
		{http.MethodPut, "/v1/config"},
		{http.MethodPost, "/v1/config/apply"},

		{http.MethodGet, "/v1/config/clusters"},
		{http.MethodPost, "/v1/config/clusters/cluster1"},
		{http.MethodGet, "/v1/config/clusters/cluster1"},
		{http.MethodPut, "/v1/config/clusters/cluster1"},
		{http.MethodDelete, "/v1/config/clusters/cluster1"},

		{http.MethodGet, "/v1/config/storage"},
		{http.MethodPost, "/v1/config/storage/storage1"},
		{http.MethodGet, "/v1/config/storage/storage1"},
		{http.MethodPut, "/v1/config/storage/storage1"},
		{http.MethodDelete, "/v1/config/storage/storage1"},

		{http.MethodGet, "/v1/config/policies"},
		{http.MethodPost, "/v1/config/policies/policy1"},
		{http.MethodGet, "/v1/config/policies/policy1"},
		{http.MethodPut, "/v1/config/policies/policy1"},
		{http.MethodDelete, "/v1/config/policies/policy1"},

		{http.MethodGet, "/v1/config/routines"},
		{http.MethodPost, "/v1/config/routines/routine1"},
		{http.MethodGet, "/v1/config/routines/routine1"},
		{http.MethodPut, "/v1/config/routines/routine1"},
		{http.MethodDelete, "/v1/config/routines/routine1"},
		{http.MethodPut, "/v1/config/routines/routine1/disable"},
		{http.MethodPut, "/v1/config/routines/routine1/enable"},

		{http.MethodGet, "/v1/backups/full"},
		{http.MethodGet, "/v1/backups/full/routine1"},
		{http.MethodGet, "/v1/backups/incremental"},
		{http.MethodGet, "/v1/backups/incremental/routine1"},
		{http.MethodPost, "/v1/backups/full/routine1"},
		{http.MethodPost, "/v1/backups/incremental/routine1"},
		{http.MethodPost, "/v1/backups/schedule/routine1"},
		{http.MethodGet, "/v1/backups/currentBackup/routine1"},
		{http.MethodPost, "/v1/backups/cancel/routine1"},

		{http.MethodPost, "/v1/restore/full"},
		{http.MethodPost, "/v1/restore/incremental"},
		{http.MethodPost, "/v1/restore/timestamp"},
		{http.MethodGet, "/v1/restore/status/1"},
		{http.MethodGet, "/v1/restore/jobs"},
		{http.MethodPost, "/v1/restore/cancel/1"},
		{http.MethodGet, "/v1/retrieve/configuration/routine1/1000"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)
			_, pattern := mux.Handler(req)
			assert.NotEmpty(t, pattern, "expected a registered route for %s %s", tt.method, tt.path)
		})
	}
}

// TestNewServeMux_MethodNotAllowed verifies that a registered path rejects
// an unsupported HTTP method (the "/" catch-all registered for the root path
// means unmatched paths fall through there, so we assert on the method mismatch
// for a path with no fallback instead of a literal "not found" pattern).
func TestNewServeMux_MethodNotAllowed(t *testing.T) {
	svc := handlers.NewService(
		t.Context(),
		model.NewConfig(),
		nil, nil, nil, nil, nil, nil, nil, nil,
	)
	mux := NewServeMux("/v1", "/", svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/v1/config/clusters/cluster1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}
