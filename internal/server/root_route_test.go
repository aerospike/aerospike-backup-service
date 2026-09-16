package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
)

// The root endpoint answers 200 at the configured context path, while unknown paths
// under it are still 404.
func TestRootRoute_HonoursContextPath(t *testing.T) {
	svc := handlers.NewService(model.NewConfig(), nil, nil, nil, nil, nil, nil, nil, nil, nil)
	mux := NewServeMux("/abs/v1", "/abs/", svc)

	for path, want := range map[string]int{
		"/abs/":              http.StatusOK,
		"/abs/no-such-route": http.StatusNotFound,
	} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, want, w.Code, path)
	}
}
