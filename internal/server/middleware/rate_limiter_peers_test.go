package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Only RemoteAddr is trusted; X-Forwarded-For does not let a client hop buckets.
func TestRateLimiter_IgnoresForwardedHeaders(t *testing.T) {
	mw := RateLimiter(&model.RateLimiterConfig{Tps: ptr.Of(1), Size: ptr.Of(1)})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	first := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	first.RemoteAddr = "198.51.100.1:1000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, first)
	require.Equal(t, http.StatusOK, rec.Code)

	// Same peer, spoofed forwarded header: must still share the exhausted bucket.
	second := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	second.RemoteAddr = "198.51.100.1:1001"
	second.Header.Set("X-Forwarded-For", "10.0.0.99")
	second.Header.Set("X-Real-IP", "10.0.0.98")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, second)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}
