package middleware

import (
	"container/list"
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
	// defaultLimiterMaxEntries caps the number of buckets the limiter keeps. A peer that
	// loses its bucket to the cap starts from a full one, which costs it a single burst;
	// keeping a bucket per source address costs the process its memory.
	defaultLimiterMaxEntries = 10_000
	// ipv6PeerPrefixBits is the prefix length IPv6 sources are grouped by. A /64 is the
	// smallest block normally assigned to one subscriber, so a client cannot claim a
	// bucket per address by cycling through its own subnet.
	ipv6PeerPrefixBits = 64
)

var allowAnyPrefix = netip.MustParsePrefix("0.0.0.0/0")

func RateLimiter(config *model.RateLimiterConfig) Middleware {
	limiters := NewIPRateLimiter(
		rate.Limit(config.GetTpsOrDefault()),
		config.GetSizeOrDefault(),
		defaultLimiterIdleTTL,
		defaultLimiterCleanupInterval,
		defaultLimiterMaxEntries,
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
	// key is the bucket this entry belongs to, kept so that evicting it from the
	// recency list also removes it from the map.
	key      netip.Prefix
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter represents a rate limiter based on an IP address.
// The number of buckets it keeps is bounded: entries idle for longer than idleTTL are
// evicted on the request path, and once the map is full the least recently used bucket
// makes room for a new peer.
type IPRateLimiter struct {
	sync.Mutex
	// limiters maps a peer to its position in recent; the element value is an *ipLimiterEntry.
	limiters map[netip.Prefix]*list.Element
	// recent orders the entries by last use, most recent at the front.
	recent          *list.List
	tokensPerSecond rate.Limit
	tokenBucketSize int
	idleTTL         time.Duration
	cleanupInterval time.Duration
	maxEntries      int
	lastCleanup     time.Time
}

// NewIPRateLimiter returns a new IPRateLimiter.
// Entries idle for longer than idleTTL are evicted on the request path, at most once per cleanupInterval.
// A cleanupInterval of zero or less disables that sweep.
// The limiter keeps at most maxEntries buckets, dropping the least recently used one to admit a new
// peer; a maxEntries of zero or less leaves the number of buckets unbounded.
func NewIPRateLimiter(tps rate.Limit, size int, idleTTL, cleanupInterval time.Duration, maxEntries int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters:        make(map[netip.Prefix]*list.Element),
		recent:          list.New(),
		tokensPerSecond: tps,
		tokenBucketSize: size,
		idleTTL:         idleTTL,
		cleanupInterval: cleanupInterval,
		maxEntries:      maxEntries,
		lastCleanup:     time.Now(),
	}
}

// Allow reports whether a request from ipAddr may proceed at the current time.
func (ipLimiter *IPRateLimiter) Allow(ipAddr netip.Addr) bool {
	return ipLimiter.getOrCreateEntry(ipAddr).limiter.Allow()
}

// peerKey maps a source address to the bucket it shares with its neighbors: an IPv4 address
// gets a bucket of its own, an IPv6 address shares one with the rest of its /64.
func peerKey(ipAddr netip.Addr) netip.Prefix {
	ipAddr = ipAddr.Unmap()

	bits := ipAddr.BitLen()
	if ipAddr.Is6() && bits > ipv6PeerPrefixBits {
		bits = ipv6PeerPrefixBits
	}

	return netip.PrefixFrom(ipAddr, bits).Masked()
}

func (ipLimiter *IPRateLimiter) getOrCreateEntry(ipAddr netip.Addr) *ipLimiterEntry {
	key := peerKey(ipAddr)

	ipLimiter.Lock()
	defer ipLimiter.Unlock()

	now := time.Now()
	ipLimiter.evictIdle(now)

	if element, exists := ipLimiter.limiters[key]; exists {
		entry := entryOf(element)
		entry.lastSeen = now
		ipLimiter.recent.MoveToFront(element)

		return entry
	}

	ipLimiter.evictLeastRecentlyUsed()

	entry := &ipLimiterEntry{
		key:      key,
		limiter:  rate.NewLimiter(ipLimiter.tokensPerSecond, ipLimiter.tokenBucketSize),
		lastSeen: now,
	}
	ipLimiter.limiters[key] = ipLimiter.recent.PushFront(entry)

	return entry
}

// evictIdle drops entries unused for idleTTL. It sweeps at most once per cleanupInterval.
// The caller must hold the lock.
func (ipLimiter *IPRateLimiter) evictIdle(now time.Time) {
	if ipLimiter.cleanupInterval <= 0 || now.Sub(ipLimiter.lastCleanup) < ipLimiter.cleanupInterval {
		return
	}
	ipLimiter.lastCleanup = now

	for _, element := range ipLimiter.limiters {
		if now.Sub(entryOf(element).lastSeen) >= ipLimiter.idleTTL {
			ipLimiter.remove(element)
		}
	}
}

// evictLeastRecentlyUsed frees a slot for one new entry once the map is full.
// The caller must hold the lock.
func (ipLimiter *IPRateLimiter) evictLeastRecentlyUsed() {
	if ipLimiter.maxEntries <= 0 {
		return
	}

	for ipLimiter.recent.Len() >= ipLimiter.maxEntries {
		oldest := ipLimiter.recent.Back()
		if oldest == nil {
			return
		}
		ipLimiter.remove(oldest)
	}
}

// remove drops an entry from both the recency list and the map. The caller must hold the lock.
func (ipLimiter *IPRateLimiter) remove(element *list.Element) {
	delete(ipLimiter.limiters, entryOf(element).key)
	ipLimiter.recent.Remove(element)
}

// entryOf reads the entry an element of the recency list carries. Only this file writes to
// that list, so the type is always an *ipLimiterEntry.
func entryOf(element *list.Element) *ipLimiterEntry {
	entry, _ := element.Value.(*ipLimiterEntry)

	return entry
}
