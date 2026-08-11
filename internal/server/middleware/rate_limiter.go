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
	)
	whitelist := newIPWhiteList(config.GetWhiteListOrDefault())

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			if whitelist.isAllowed(ip) {
				next.ServeHTTP(w, r)
				return
			}

			limiter := limiters.GetLimiter(ip)
			if !limiter.Allow() {
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type IPWhiteList struct {
	addresses map[string]*netip.Addr
	networks  []*netip.Prefix
	allowAny  bool
}

func newIPWhiteList(ipList []string) *IPWhiteList {
	addresses := make(map[string]*netip.Addr)
	networks := make([]*netip.Prefix, 0)
	var allowAny bool

	for _, ip := range ipList {
		network, err := netip.ParsePrefix(ip)
		if err == nil {
			if network == allowAnyPrefix {
				allowAny = true
				continue
			}
			networks = append(networks, &network)
			continue
		}

		ipAddr, err := netip.ParseAddr(ip)
		if err != nil {
			// Config validation should catch malformed entries.
			// Keep runtime safe in case config arrives from another source.
			slog.Warn("Ignoring invalid whitelist entry", slog.String("entry", ip))
			continue
		}
		addresses[ip] = &ipAddr
	}

	return &IPWhiteList{
		addresses: addresses,
		networks:  networks,
		allowAny:  allowAny,
	}
}

func (wl *IPWhiteList) isAllowed(ip string) bool {
	if wl.allowAny {
		return true
	}
	ipAddr, err := netip.ParseAddr(ip)
	if err != nil {
		slog.Warn("Invalid client IP")
		return false
	}
	_, ok := wl.addresses[ip]
	if ok {
		return true
	}

	for _, network := range wl.networks {
		if network.Contains(ipAddr) {
			return true
		}
	}

	return false
}

// IPAddress represents an IP address string.
type IPAddress string

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
	limiters        map[IPAddress]*ipLimiterEntry
	tokensPerSecond rate.Limit
	tokenBucketSize int
	now             func() time.Time
	idleTTL         time.Duration
	cleanupTicker   ticker
	stopCh          chan struct{}
}

// NewIPRateLimiter returns a new IPRateLimiter.
func NewIPRateLimiter(tps rate.Limit, size int) *IPRateLimiter {
	return newIPRateLimiter(
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
	tps rate.Limit,
	size int,
	idleTTL time.Duration,
	cleanupInterval time.Duration,
	now func() time.Time,
	newTicker func(time.Duration) ticker,
) *IPRateLimiter {
	ipLimiter := &IPRateLimiter{
		limiters:        make(map[IPAddress]*ipLimiterEntry),
		tokensPerSecond: tps,
		tokenBucketSize: size,
		now:             now,
		idleTTL:         idleTTL,
		stopCh:          make(chan struct{}),
	}

	if cleanupInterval > 0 {
		ipLimiter.cleanupTicker = newTicker(cleanupInterval)
		go ipLimiter.cleanupLoop()
	}

	return ipLimiter
}

// AddLimiter creates a new rate limiter and adds it to the limiters map,
// using the IP address as the key.
func (ipLimiter *IPRateLimiter) AddLimiter(ipAddr string) *rate.Limiter {
	ipLimiter.Lock()
	defer ipLimiter.Unlock()

	limiter := rate.NewLimiter(ipLimiter.tokensPerSecond, ipLimiter.tokenBucketSize)

	ipLimiter.limiters[IPAddress(ipAddr)] = &ipLimiterEntry{
		limiter:  limiter,
		lastSeen: ipLimiter.now(),
	}

	return limiter
}

// GetLimiter returns the rate limiter for the provided IP address if it exists.
// Otherwise calls AddLimiter to add a new limiter to the map.
func (ipLimiter *IPRateLimiter) GetLimiter(ipAddr string) *rate.Limiter {
	ipLimiter.Lock()
	entry, exists := ipLimiter.limiters[IPAddress(ipAddr)]

	if !exists {
		ipLimiter.Unlock()
		return ipLimiter.AddLimiter(ipAddr)
	}
	entry.lastSeen = ipLimiter.now()

	ipLimiter.Unlock()

	return entry.limiter
}

func (ipLimiter *IPRateLimiter) cleanupLoop() {
	for {
		select {
		case now := <-ipLimiter.cleanupTicker.C():
			ipLimiter.evictIdle(now)
		case <-ipLimiter.stopCh:
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

func (ipLimiter *IPRateLimiter) stop() {
	select {
	case <-ipLimiter.stopCh:
		return
	default:
		close(ipLimiter.stopCh)
	}
	if ipLimiter.cleanupTicker != nil {
		ipLimiter.cleanupTicker.Stop()
	}
}
