package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"golang.org/x/time/rate"
)

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
		if strings.HasPrefix(ip, "0.0.0.0") {
			allowAny = true
			continue
		}
		network, err := netip.ParsePrefix(ip)
		if err != nil {
			ipAddr, err := netip.ParseAddr(ip)
			if err != nil {
				panic("invalid ip configuration: " + ip)
			}
			addresses[ip] = &ipAddr
		} else {
			networks = append(networks, &network)
		}
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
		slog.Warn("Invalid client IP", slog.String("ip", ip))
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

// IPRateLimiter represents a rate limiter based on an IP address.
type IPRateLimiter struct {
	sync.Mutex
	limiters        map[IPAddress]*rate.Limiter
	tokensPerSecond rate.Limit
	tokenBucketSize int
}

// NewIPRateLimiter returns a new IPRateLimiter.
func NewIPRateLimiter(tps rate.Limit, size int) *IPRateLimiter {
	ipLimiter := &IPRateLimiter{
		limiters:        make(map[IPAddress]*rate.Limiter),
		tokensPerSecond: tps,
		tokenBucketSize: size,
	}

	return ipLimiter
}

// AddLimiter creates a new rate limiter and adds it to the limiters map,
// using the IP address as the key.
func (ipLimiter *IPRateLimiter) AddLimiter(ipAddr string) *rate.Limiter {
	ipLimiter.Lock()
	defer ipLimiter.Unlock()

	limiter := rate.NewLimiter(ipLimiter.tokensPerSecond, ipLimiter.tokenBucketSize)

	ipLimiter.limiters[IPAddress(ipAddr)] = limiter

	return limiter
}

// GetLimiter returns the rate limiter for the provided IP address if it exists.
// Otherwise calls AddLimiter to add a new limiter to the map.
func (ipLimiter *IPRateLimiter) GetLimiter(ipAddr string) *rate.Limiter {
	ipLimiter.Lock()
	limiter, exists := ipLimiter.limiters[IPAddress(ipAddr)]

	if !exists {
		ipLimiter.Unlock()
		return ipLimiter.AddLimiter(ipAddr)
	}

	ipLimiter.Unlock()

	return limiter
}
