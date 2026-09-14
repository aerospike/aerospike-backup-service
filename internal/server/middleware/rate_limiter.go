package middleware

import (
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

func RateLimiter(config *model.RateLimiterConfig) Middleware {
	limiters := NewIPRateLimiter(
		rate.Limit(config.GetTpsOrDefault()),
		config.GetSizeOrDefault(),
		defaultLimiterIdleTTL,
		defaultLimiterCleanupInterval,
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

// IPRateLimiter represents a rate limiter based on an IP address.
type IPRateLimiter struct {
	sync.Mutex
	limiters        map[netip.Addr]*ipLimiterEntry
	tokensPerSecond rate.Limit
	tokenBucketSize int
	idleTTL         time.Duration
	cleanupInterval time.Duration
	lastCleanup     time.Time
}

// NewIPRateLimiter returns a new IPRateLimiter.
// Entries idle for longer than idleTTL are evicted on the request path, at most once per cleanupInterval.
// A cleanupInterval of zero or less disables eviction.
func NewIPRateLimiter(tps rate.Limit, size int, idleTTL, cleanupInterval time.Duration) *IPRateLimiter {
	return &IPRateLimiter{
		limiters:        make(map[netip.Addr]*ipLimiterEntry),
		tokensPerSecond: tps,
		tokenBucketSize: size,
		idleTTL:         idleTTL,
		cleanupInterval: cleanupInterval,
		lastCleanup:     time.Now(),
	}
}

// Allow reports whether a request from ipAddr may proceed at the current time.
func (ipLimiter *IPRateLimiter) Allow(ipAddr netip.Addr) bool {
	return ipLimiter.getOrCreateEntry(ipAddr).limiter.Allow()
}

func (ipLimiter *IPRateLimiter) getOrCreateEntry(ipAddr netip.Addr) *ipLimiterEntry {
	ipLimiter.Lock()
	defer ipLimiter.Unlock()

	now := time.Now()
	ipLimiter.evictIdle(now)

	entry, exists := ipLimiter.limiters[ipAddr]
	if !exists {
		entry = &ipLimiterEntry{
			limiter: rate.NewLimiter(ipLimiter.tokensPerSecond, ipLimiter.tokenBucketSize),
		}
		ipLimiter.limiters[ipAddr] = entry
	}
	entry.lastSeen = now

	return entry
}

// evictIdle drops entries unused for idleTTL. It sweeps at most once per cleanupInterval.
// The caller must hold the lock.
func (ipLimiter *IPRateLimiter) evictIdle(now time.Time) {
	if ipLimiter.cleanupInterval <= 0 || now.Sub(ipLimiter.lastCleanup) < ipLimiter.cleanupInterval {
		return
	}
	ipLimiter.lastCleanup = now

	for ip, entry := range ipLimiter.limiters {
		if now.Sub(entry.lastSeen) >= ipLimiter.idleTTL {
			delete(ipLimiter.limiters, ip)
		}
	}
}
