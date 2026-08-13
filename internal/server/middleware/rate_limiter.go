package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"golang.org/x/time/rate"
)

const (
	defaultLimiterIdleTTL         = 10 * time.Minute
	defaultLimiterCleanupInterval = 1 * time.Minute
)

var allowAnyPrefix = netip.MustParsePrefix("0.0.0.0/0")

func RateLimiter(ctx context.Context, config *model.RateLimiterConfig) Middleware {
	limiters := NewIPRateLimiter(
		ctx,
		rate.Limit(config.GetTpsOrDefault()),
		config.GetSizeOrDefault(),
	)
	whitelist := newIPWhiteList(config.GetWhiteListOrDefault())

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ipStr, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			ipAddr, err := netip.ParseAddr(ipStr)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			ipAddr = ipAddr.Unmap()

			if whitelist.isAllowed(ipAddr) || limiters.Allow(ipAddr) {
				next.ServeHTTP(w, r)
				return
			}

			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		})
	}
}

type IPWhiteList struct {
	addresses map[netip.Addr]struct{}
	networks  []netip.Prefix
	allowAny  bool
}

func newIPWhiteList(ipList []string) *IPWhiteList {
	addresses := make(map[netip.Addr]struct{})
	networks := make([]netip.Prefix, 0)
	var allowAny bool

	for _, ip := range ipList {
		network, err := netip.ParsePrefix(ip)
		if err == nil {
			if network == allowAnyPrefix {
				allowAny = true
				continue
			}
			networks = append(networks, network)
			continue
		}

		ipAddr, err := netip.ParseAddr(ip)
		if err != nil {
			// Config validation should catch malformed entries.
			// Keep runtime safe in case config arrives from another source.
			slog.Warn("Ignoring invalid whitelist entry", slog.String("entry", ip))
			continue
		}
		addresses[ipAddr.Unmap()] = struct{}{}
	}

	return &IPWhiteList{
		addresses: addresses,
		networks:  networks,
		allowAny:  allowAny,
	}
}

func (wl *IPWhiteList) isAllowed(ip netip.Addr) bool {
	if wl.allowAny {
		return true
	}

	_, ok := wl.addresses[ip]
	if ok {
		return true
	}

	for _, network := range wl.networks {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

type ipLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ticker interface {
	C() <-chan time.Time
	Stop()
}

type realTicker struct {
	*time.Ticker
}

func (t *realTicker) C() <-chan time.Time {
	return t.Ticker.C
}

// IPRateLimiter represents a rate limiter based on an IP address.
type IPRateLimiter struct {
	sync.Mutex
	limiters        map[netip.Addr]*ipLimiterEntry
	tokensPerSecond rate.Limit
	tokenBucketSize int
	now             func() time.Time
	idleTTL         time.Duration
	cleanupTicker   ticker
}

// NewIPRateLimiter returns a new IPRateLimiter.
func NewIPRateLimiter(ctx context.Context, tps rate.Limit, size int) *IPRateLimiter {
	return newIPRateLimiter(
		ctx,
		tps,
		size,
		defaultLimiterIdleTTL,
		defaultLimiterCleanupInterval,
		time.Now,
		func(d time.Duration) ticker {
			return &realTicker{Ticker: time.NewTicker(d)}
		},
	)
}

func newIPRateLimiter(
	ctx context.Context,
	tps rate.Limit,
	size int,
	idleTTL time.Duration,
	cleanupInterval time.Duration,
	now func() time.Time,
	newTicker func(time.Duration) ticker,
) *IPRateLimiter {
	ipLimiter := &IPRateLimiter{
		limiters:        make(map[netip.Addr]*ipLimiterEntry),
		tokensPerSecond: tps,
		tokenBucketSize: size,
		now:             now,
		idleTTL:         idleTTL,
	}

	if cleanupInterval > 0 {
		ipLimiter.cleanupTicker = newTicker(cleanupInterval)
		go ipLimiter.cleanupLoop(ctx)
	}

	return ipLimiter
}

// Allow reports whether a request from ipAddr may proceed at the current time.
func (ipLimiter *IPRateLimiter) Allow(ipAddr netip.Addr) bool {
	return ipLimiter.getOrCreateEntry(ipAddr).limiter.Allow()
}

func (ipLimiter *IPRateLimiter) getOrCreateEntry(ipAddr netip.Addr) *ipLimiterEntry {
	ipLimiter.Lock()
	defer ipLimiter.Unlock()

	entry, exists := ipLimiter.limiters[ipAddr]
	if !exists {
		limiter := rate.NewLimiter(ipLimiter.tokensPerSecond, ipLimiter.tokenBucketSize)
		entry = &ipLimiterEntry{
			limiter:  limiter,
			lastSeen: ipLimiter.now(),
		}
		ipLimiter.limiters[ipAddr] = entry
	} else {
		entry.lastSeen = ipLimiter.now()
	}

	return entry
}

func (ipLimiter *IPRateLimiter) cleanupLoop(ctx context.Context) {
	for {
		select {
		case now := <-ipLimiter.cleanupTicker.C():
			ipLimiter.evictIdle(now)
		case <-ctx.Done():
			ipLimiter.cleanupTicker.Stop()
			return
		}
	}
}

func (ipLimiter *IPRateLimiter) evictIdle(now time.Time) {
	ipLimiter.Lock()
	defer ipLimiter.Unlock()

	for ip, entry := range ipLimiter.limiters {
		if now.Sub(entry.lastSeen) >= ipLimiter.idleTTL {
			delete(ipLimiter.limiters, ip)
		}
	}
}
