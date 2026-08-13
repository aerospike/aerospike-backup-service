package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
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

func TestIPRateLimiter_EvictsIdleEntriesOnTick(t *testing.T) {
	const (
		idleTTL         = 150 * time.Millisecond
		cleanupInterval = 50 * time.Millisecond
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	limiter := NewIPRateLimiter(ctx, rate.Limit(1), 1, idleTTL, cleanupInterval)

	limiter.getOrCreateEntry(netip.MustParseAddr("10.0.0.1"))
	limiter.getOrCreateEntry(netip.MustParseAddr("10.0.0.2"))

	time.Sleep(80 * time.Millisecond)
	limiter.getOrCreateEntry(netip.MustParseAddr("10.0.0.1"))

	require.Eventually(t, func() bool {
		limiter.Lock()
		defer limiter.Unlock()

		_, firstExists := limiter.limiters[netip.MustParseAddr("10.0.0.1")]
		_, secondExists := limiter.limiters[netip.MustParseAddr("10.0.0.2")]
		return firstExists && !secondExists
	}, 2*time.Second, 10*time.Millisecond)
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
	}), bodyReader, RateLimiter(t.Context(), &model.RateLimiterConfig{
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
