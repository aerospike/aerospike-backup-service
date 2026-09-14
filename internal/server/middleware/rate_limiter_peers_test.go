package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// The per-IP limiter map must stay bounded: a peer cycling through source addresses (an IPv6
// /64, for example) must not be able to grow it without limit.
func TestRateLimiter_BoundsEntriesPerPeer(t *testing.T) {
	t.Skip("BKRS-417: the per-IP limiter map has no size cap")

	limiter := NewIPRateLimiter(rate.Limit(1), 1, defaultLimiterIdleTTL, 0)

	const distinct = 50_000
	for i := range distinct {
		addr := netip.MustParseAddr(fmt.Sprintf("2001:db8::%x:%x", i>>16, i&0xffff))
		limiter.Allow(addr)
	}

	limiter.Lock()
	size := len(limiter.limiters)
	limiter.Unlock()

	assert.Less(t, size, distinct, "entries must be capped below the number of distinct peers")
}

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
