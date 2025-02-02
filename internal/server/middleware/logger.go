package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// RequestLogger returns a middleware that logs request details using provided logger.
func RequestLogger(logger *slog.Logger, skipPaths []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip logging for specified paths
			for _, path := range skipPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			start := time.Now()
			rw := newResponseWriter(w)

			// Process request
			next.ServeHTTP(rw, r)

			// Log request details
			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"duration", time.Since(start).Milliseconds(),
				"ip", r.RemoteAddr,
			)
		})
	}
}

// responseWriter captures the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
	}
}

// WriteHeader captures the status code and calls the underlying ResponseWriter.
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.status = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}
