package middleware

import (
	"net"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v2/internal/util"
)

func RateLimiter(rateLimiter *util.IPRateLimiter, whiteList *util.IPWhiteList,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			if !whiteList.IsAllowed(ip) {
				limiter := rateLimiter.GetLimiter(ip)
				if !limiter.Allow() {
					http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
