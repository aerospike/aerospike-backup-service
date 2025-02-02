package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/middleware"
	"github.com/aerospike/aerospike-backup-service/v3/internal/util"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"golang.org/x/time/rate"
)

const (
	restAPIVersion = "v1"
)

// HTTPServer is the backup service HTTP server wrapper.
type HTTPServer struct {
	server *http.Server
}

// NewHTTPServer returns a new instance of HTTPServer.
func NewHTTPServer(serverConfig *model.HTTPServerConfig, h *handlers.Service, logger *slog.Logger) *HTTPServer {
	addr := fmt.Sprintf("%s:%d", serverConfig.GetAddressOrDefault(), serverConfig.GetPortOrDefault())

	rateLimiterConfig := serverConfig.GetRateOrDefault()
	rateLimiter := util.NewIPRateLimiter(
		rate.Limit(rateLimiterConfig.GetTpsOrDefault()),
		rateLimiterConfig.GetSizeOrDefault(),
	)
	whitelist := util.NewIPWhiteList(rateLimiterConfig.GetWhiteListOrDefault())

	// Create router
	mux := NewServeMux(
		fmt.Sprintf("/%s", restAPIVersion),
		"/",
		h,
	)

	// Configure logging middleware
	loggerOpts := &middleware.LoggerOptions{
		SkipPaths: []string{"/health", "/ready", "/metrics"},
	}

	// Apply middleware chain
	handler := middleware.WithRequestLogging(logger, loggerOpts)(
		middleware.RateLimiter(rateLimiter, whitelist)(mux),
	)

	return &HTTPServer{
		server: &http.Server{
			Addr:              addr,
			ReadHeaderTimeout: serverConfig.GetTimeoutOrDefault(),
			Handler:           handler,
		},
	}
}

// Start starts the HTTP server.
func (s *HTTPServer) Start() {
	err := s.server.ListenAndServe()
	if err != nil && strings.Contains(err.Error(), "Server closed") {
		slog.Info(err.Error())
	} else {
		panic(err)
	}
}

// Shutdown shutdowns the HTTP server.
func (s *HTTPServer) Shutdown() error {
	return s.server.Shutdown(context.Background())
}
