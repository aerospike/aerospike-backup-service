package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const maxLogSize = 10_000 // Maximum size of request body to log in bytes.

// RequestLogger returns a middleware that logs request details using provided logger.
func RequestLogger(logger *slog.Logger, skipPaths []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip logging for specified paths
			trimmedPath := strings.TrimLeft(r.URL.Path, "/")
			for _, path := range skipPaths {
				if strings.HasPrefix(trimmedPath, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			body, err := readRequestBody(r)
			if err != nil {
				logger.Error("failed to read request body", "error", err)
				http.Error(w, "Failed to read request body: "+err.Error(), http.StatusInternalServerError)
				return
			}

			rw := newCapturingResponseWriter(w)
			start := time.Now()

			next.ServeHTTP(rw, r)

			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rw.status),
				slog.Duration("duration", time.Since(start)),
				slog.String("ip", r.RemoteAddr),
			}

			if len(body) > 0 {
				attrs = append(attrs, slog.String("request_body", string(body[:min(len(body), maxLogSize)])))
			}

			// Log based on response status
			var (
				msg      string
				logLevel slog.Level
			)
			switch {
			case rw.status >= 500:
				msg = "request error"
				logLevel = slog.LevelError
			case rw.status >= 400:
				msg = "request failed"
				logLevel = slog.LevelWarn
			default:
				msg = "request success"
				logLevel = slog.LevelInfo
			}

			if rw.errorMsg != "" {
				attrs = append(attrs, slog.String("error", rw.errorMsg))
			}

			logger.LogAttrs(r.Context(), logLevel, msg, attrs...)
		})
	}
}

// readRequestBody reads the request body and resets is, so it can be used by subsequent handlers.
func readRequestBody(r *http.Request) ([]byte, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return bodyBytes, nil
}

// capturingResponseWriter captures the status code and error message.
type capturingResponseWriter struct {
	http.ResponseWriter
	status   int
	errorMsg string
}

func newCapturingResponseWriter(w http.ResponseWriter) *capturingResponseWriter {
	return &capturingResponseWriter{
		ResponseWriter: w,
	}
}

// WriteHeader captures the status code and calls the underlying ResponseWriter.
func (rw *capturingResponseWriter) WriteHeader(statusCode int) {
	rw.status = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures error messages from http.Error calls.
func (rw *capturingResponseWriter) Write(b []byte) (int, error) {
	if rw.status >= 400 { // Response body contains an error message
		rw.errorMsg = string(b)
	}

	return rw.ResponseWriter.Write(b)
}
