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

// The per-IP limiter map stays bounded: a peer cycling through source addresses (an IPv6
// /64, for example) must not be able to grow it without limit.
func TestRateLimiter_BoundsEntriesPerPeer(t *testing.T) {
	const distinct = 50_000

	limiter := NewIPRateLimiter(rate.Limit(1), 1, defaultLimiterIdleTTL, 0, defaultLimiterMaxEntries)
	for i := range distinct {
		limiter.Allow(netip.MustParseAddr(fmt.Sprintf("2001:db8::%x:%x", i>>16, i&0xffff)))
	}

	limiter.Lock()
	defer limiter.Unlock()
	assert.Len(t, limiter.limiters, 1, "a single /64 must share one bucket")
}

// Addresses in the same IPv6 /64 share a bucket; a different /64 gets its own.
func TestIPRateLimiter_GroupsIPv6BySlash64(t *testing.T) {
	limiter := NewIPRateLimiter(rate.Limit(1), 1, defaultLimiterIdleTTL, 0, defaultLimiterMaxEntries)

	require.True(t, limiter.Allow(netip.MustParseAddr("2001:db8::1")))
	assert.False(t, limiter.Allow(netip.MustParseAddr("2001:db8::2")), "same /64 shares the bucket")
	assert.True(t, limiter.Allow(netip.MustParseAddr("2001:db8:0:1::1")), "a different /64 has its own bucket")
}

// IPv4 peers keep a bucket each: grouping applies to IPv6 only.
func TestIPRateLimiter_KeepsIPv4PerAddress(t *testing.T) {
	limiter := NewIPRateLimiter(rate.Limit(1), 1, defaultLimiterIdleTTL, 0, defaultLimiterMaxEntries)

	require.True(t, limiter.Allow(netip.MustParseAddr("198.51.100.1")))
	assert.False(t, limiter.Allow(netip.MustParseAddr("198.51.100.1")))
	assert.True(t, limiter.Allow(netip.MustParseAddr("198.51.100.2")))
}

// Once the map is full, the least recently used bucket makes room for a new peer and the
// buckets still in use survive.
func TestIPRateLimiter_EvictsLeastRecentlyUsedWhenFull(t *testing.T) {
	limiter := NewIPRateLimiter(rate.Limit(1), 1, defaultLimiterIdleTTL, 0, 2)

	first := netip.MustParseAddr("198.51.100.1")
	second := netip.MustParseAddr("198.51.100.2")
	third := netip.MustParseAddr("198.51.100.3")

	limiter.getOrCreateEntry(first)
	limiter.getOrCreateEntry(second)
	limiter.getOrCreateEntry(first) // first is now the most recently used
	limiter.getOrCreateEntry(third)

	limiter.Lock()
	defer limiter.Unlock()
	assert.Len(t, limiter.limiters, 2)
	assert.NotContains(t, limiter.limiters, peerKey(second), "least recently used entry survived")
	assert.Contains(t, limiter.limiters, peerKey(first))
	assert.Contains(t, limiter.limiters, peerKey(third))
	assert.Equal(t, len(limiter.limiters), limiter.recent.Len(), "recency list out of sync with the map")
}
