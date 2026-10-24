package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestIPWhiteList_AllowAnyRequiresExplicitCIDR(t *testing.T) {
	t.Run("exact allow any cidr", func(t *testing.T) {
		wl := newIPWhiteList([]string{"0.0.0.0/0"})
		require.True(t, wl.isAllowed(netip.MustParseAddr("203.0.113.77")))
	})

	t.Run("single 0.0.0.0 is not global bypass", func(t *testing.T) {
		wl := newIPWhiteList([]string{"0.0.0.0"})
		require.True(t, wl.isAllowed(netip.MustParseAddr("0.0.0.0")))
		require.False(t, wl.isAllowed(netip.MustParseAddr("203.0.113.77")))
	})
}

func TestIPRateLimiter_EvictsIdleEntriesOnRequest(t *testing.T) {
	const (
		idleTTL         = time.Minute
		cleanupInterval = time.Minute
	)

	limiter := NewIPRateLimiter(rate.Limit(1), 1, idleTTL, cleanupInterval, defaultLimiterMaxEntries)
	active := netip.MustParseAddr("10.0.0.1")
	idle := netip.MustParseAddr("10.0.0.2")

	limiter.getOrCreateEntry(active)
	limiter.getOrCreateEntry(idle)

	// Age the second entry past its TTL and make a sweep due on the next request.
	limiter.Lock()
	entryOf(limiter.limiters[peerKey(idle)]).lastSeen = time.Now().Add(-2 * idleTTL)
	limiter.lastCleanup = time.Now().Add(-2 * cleanupInterval)
	limiter.Unlock()

	limiter.getOrCreateEntry(active)

	limiter.Lock()
	defer limiter.Unlock()
	require.NotContains(t, limiter.limiters, peerKey(idle), "idle entry survived the sweep")
	require.Contains(t, limiter.limiters, peerKey(active), "active entry was evicted")
	require.Equal(t, len(limiter.limiters), limiter.recent.Len(), "recency list out of sync with the map")
}

func TestIPRateLimiter_SweepsAtMostOncePerInterval(t *testing.T) {
	const (
		idleTTL         = time.Minute
		cleanupInterval = time.Minute
	)

	limiter := NewIPRateLimiter(rate.Limit(1), 1, idleTTL, cleanupInterval, defaultLimiterMaxEntries)
	active := netip.MustParseAddr("10.0.0.1")
	idle := netip.MustParseAddr("10.0.0.2")

	limiter.getOrCreateEntry(active)
	limiter.getOrCreateEntry(idle)

	// Idle past its TTL, but the interval since the last sweep has not elapsed.
	limiter.Lock()
	entryOf(limiter.limiters[peerKey(idle)]).lastSeen = time.Now().Add(-2 * idleTTL)
	limiter.Unlock()

	limiter.getOrCreateEntry(active)

	limiter.Lock()
	defer limiter.Unlock()
	require.Contains(t, limiter.limiters, peerKey(idle), "swept before the interval elapsed")
}

func TestRateLimiter_RunsBeforeBodyReaderMiddleware(t *testing.T) {
	var bodyReadCount int

	bodyReader := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			bodyReadCount++
			r.Body = io.NopCloser(bytes.NewBuffer(body))
			next.ServeHTTP(w, r)
		})
	}

	tps := 1
	size := 1
	handler := Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), bodyReader, RateLimiter(&model.RateLimiterConfig{
		Tps:       &tps,
		Size:      &size,
		WhiteList: []string{},
	}))

	req1 := httptest.NewRequestWithContext(t.Context(),
		http.MethodPost, "/backup", bytes.NewBufferString("first"))
	req1.RemoteAddr = "192.0.2.1:1234"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)
	require.Equal(t, 1, bodyReadCount)

	req2 := httptest.NewRequestWithContext(t.Context(),
		http.MethodPost, "/backup", bytes.NewBufferString("second"))
	req2.RemoteAddr = "192.0.2.1:1234"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusTooManyRequests, rec2.Code)
	require.Equal(t, 1, bodyReadCount)
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
