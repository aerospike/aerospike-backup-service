package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unconfiguredServiceConfig is the default deployment shape this suite protects: an empty file, so
// no tls block and no auth block are present and every other setting falls back to its default.
const unconfiguredServiceConfig = ``

// newUnconfiguredService builds the real object graph from unconfiguredServiceConfig and returns
// the HTTP handler that cmd/backup serves. Requests go through the full middleware chain, not just
// the router, because that is where request-scoped policy such as authentication is applied.
func newUnconfiguredService(t *testing.T) http.Handler {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte(unconfiguredServiceConfig), 0o600))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	components, err := app.InitComponents(ctx, configPath, false)
	require.NoError(t, err)

	return components.ServerHTTP
}

func serve(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	return w
}

// TestUnconfiguredService_APIContract pins the status code and response body of each route when the
// service runs without tls and without auth.
//
// The test is in package server_test rather than package server on purpose: it drives the fully
// wired handler from app.InitComponents, and app already imports server, so an in-package test
// importing app would be an import cycle. An external test package keeps that dependency one-way.
//
// Every security feature is opt-in, so an unconfigured service must keep answering exactly as it
// does today. A failure here is a release blocker: fix the regression rather than updating the
// expectations below.
//
// The table covers every route registered in NewServeMux. Each row hits a fresh process so a
// mutating request cannot change the baseline of another row. Requests are chosen to reach a
// verdict without a live Aerospike cluster or object store, so the table stays deterministic and
// runs in the default `go test ./...` suite.
//
// Routes whose body changes between builds or between calls (/version, /metrics, /api-docs/) are
// covered by TestUnconfiguredService_VolatileResponses instead.
func TestUnconfiguredService_APIContract(t *testing.T) {
	tests := []struct {
		method     string
		path       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{http.MethodGet, "/", "", http.StatusOK, ""},
		{http.MethodGet, "/health", "", http.StatusOK, "Ok"},
		{http.MethodGet, "/ready", "", http.StatusOK, "Ok"},

		{http.MethodGet, "/v1/config", "", http.StatusOK, "{}"},
		{http.MethodPut, "/v1/config", "{}", http.StatusOK, ""},
		{http.MethodPost, "/v1/config/apply", "", http.StatusOK, ""},

		{http.MethodGet, "/v1/config/clusters", "", http.StatusOK, "{}"},
		{
			http.MethodPost, "/v1/config/clusters/cluster1", "{}", http.StatusBadRequest,
			"invalid JSON payload: seed nodes are not specified\n",
		},
		{
			http.MethodGet, "/v1/config/clusters/cluster1", "", http.StatusNotFound,
			"cluster \"cluster1\" not found\n",
		},
		{
			http.MethodPut, "/v1/config/clusters/cluster1", "{}", http.StatusBadRequest,
			"invalid JSON payload: seed nodes are not specified\n",
		},
		{
			http.MethodDelete, "/v1/config/clusters/cluster1", "", http.StatusBadRequest,
			"invalid request: failed to update configuration: delete Aerospike cluster \"cluster1\": item not found\n",
		},

		{http.MethodGet, "/v1/config/storage", "", http.StatusOK, "{}"},
		{
			http.MethodPost, "/v1/config/storage/storage1", "{}", http.StatusBadRequest,
			"invalid JSON payload: no storage type specified\n",
		},
		{
			http.MethodGet, "/v1/config/storage/storage1", "", http.StatusNotFound,
			"storage \"storage1\" not found\n",
		},
		{
			http.MethodPut, "/v1/config/storage/storage1", "{}", http.StatusBadRequest,
			"invalid JSON payload: no storage type specified\n",
		},
		{
			http.MethodDelete, "/v1/config/storage/storage1", "", http.StatusBadRequest,
			"invalid request: failed to update configuration: delete storage \"storage1\": item not found\n",
		},

		{http.MethodGet, "/v1/config/policies", "", http.StatusOK, "{}"},
		{http.MethodPost, "/v1/config/policies/policy1", "{}", http.StatusCreated, ""},
		{
			http.MethodGet, "/v1/config/policies/policy1", "", http.StatusNotFound,
			"policy \"policy1\" not found\n",
		},
		{
			http.MethodPut, "/v1/config/policies/policy1", "{}", http.StatusBadRequest,
			"invalid request: failed to update configuration: update backup policy \"policy1\": item not found\n",
		},
		{
			http.MethodDelete, "/v1/config/policies/policy1", "", http.StatusBadRequest,
			"invalid request: failed to update configuration: delete backup policy \"policy1\": item not found\n",
		},

		{http.MethodGet, "/v1/config/routines", "", http.StatusOK, "{}"},
		{
			http.MethodPost, "/v1/config/routines/routine1", "{}", http.StatusBadRequest,
			"invalid JSON payload: empty field validation error: \"source-cluster\" required\n",
		},
		{
			http.MethodGet, "/v1/config/routines/routine1", "", http.StatusNotFound,
			"routine \"routine1\" not found\n",
		},
		{
			http.MethodPut, "/v1/config/routines/routine1", "{}", http.StatusBadRequest,
			"invalid JSON payload: empty field validation error: \"source-cluster\" required\n",
		},
		{
			http.MethodDelete, "/v1/config/routines/routine1", "", http.StatusBadRequest,
			"invalid request: failed to update configuration: delete backup routine \"routine1\": item not found\n",
		},
		{
			http.MethodPut, "/v1/config/routines/routine1/disable", "", http.StatusNotFound,
			"routine \"routine1\" not found\n",
		},
		{
			http.MethodPut, "/v1/config/routines/routine1/enable", "", http.StatusNotFound,
			"routine \"routine1\" not found\n",
		},

		{http.MethodGet, "/v1/backups/full", "", http.StatusOK, "{}"},
		{
			http.MethodGet, "/v1/backups/full/routine1", "", http.StatusNotFound,
			"routine \"routine1\" not found\n",
		},
		{http.MethodGet, "/v1/backups/incremental", "", http.StatusOK, "{}"},
		{
			http.MethodGet, "/v1/backups/incremental/routine1", "", http.StatusNotFound,
			"routine \"routine1\" not found\n",
		},
		{
			http.MethodPost, "/v1/backups/full/routine1", "", http.StatusNotFound,
			"routine \"routine1\" not found\n",
		},
		{
			http.MethodPost, "/v1/backups/incremental/routine1", "", http.StatusNotFound,
			"routine \"routine1\" not found\n",
		},
		{
			http.MethodPost, "/v1/backups/schedule/routine1", "", http.StatusNotFound,
			"routine \"routine1\" not found\n",
		},
		{
			http.MethodGet, "/v1/backups/currentBackup/routine1", "", http.StatusNotFound,
			"routine \"routine1\" not found\n",
		},
		{
			http.MethodPost, "/v1/backups/cancel/routine1", "", http.StatusNotFound,
			"routine \"routine1\" not found\n",
		},

		{
			http.MethodPost, "/v1/restore/full", "{}", http.StatusBadRequest,
			"invalid request: empty field validation error: \"backup-data-path\" required\n",
		},
		{
			http.MethodPost, "/v1/restore/incremental", "{}", http.StatusBadRequest,
			"invalid request: empty field validation error: \"backup-data-path\" required\n",
		},
		{
			http.MethodPost, "/v1/restore/timestamp", "{}", http.StatusBadRequest,
			"invalid request: empty field validation error: \"time\" required\n",
		},
		{
			http.MethodGet, "/v1/restore/status/1", "", http.StatusNotFound,
			"job '\\x01' not found\n",
		},
		{http.MethodGet, "/v1/restore/jobs", "", http.StatusOK, "{}"},
		{
			http.MethodPost, "/v1/restore/cancel/1", "", http.StatusNotFound,
			"job '\\x01' not found\n",
		},
		{
			http.MethodGet, "/v1/retrieve/configuration/routine1/1000", "", http.StatusNotFound,
			"routine \"routine1\" not found\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			w := serve(t, newUnconfiguredService(t), tt.method, tt.path, tt.body)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantBody, w.Body.String())
		})
	}
}

// TestUnconfiguredService_VolatileResponses covers routes whose body cannot be compared verbatim:
// /version embeds the build stamp, /metrics embeds live process counters, and /api-docs/ serves
// generated Swagger UI. Their contract is still that an unauthenticated caller gets a served
// response, so the status and the response shape are asserted instead of the exact bytes.
func TestUnconfiguredService_VolatileResponses(t *testing.T) {
	handler := newUnconfiguredService(t)

	t.Run("version is a JSON object with the same keys", func(t *testing.T) {
		w := serve(t, handler, http.MethodGet, "/version", "")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.ElementsMatch(t, []string{"version", "commit", "build-time"}, keysOf(response))
	})

	t.Run("metrics are exposed in Prometheus text format without credentials", func(t *testing.T) {
		w := serve(t, handler, http.MethodGet, "/metrics", "")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
		// Backup metrics carry labels, so a service with no routines exports none of them yet.
		// The always-present handler counter is enough to prove the endpoint really served a scrape.
		assert.Contains(t, w.Body.String(), "promhttp_metric_handler_requests_total")
	})

	t.Run("api-docs are served as Swagger UI without credentials", func(t *testing.T) {
		w := serve(t, handler, http.MethodGet, "/api-docs/", "")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Swagger UI")
	})
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	return keys
}
