package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/middleware"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
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

	// Create router
	mux := NewServeMux(
		fmt.Sprintf("/%s", restAPIVersion),
		"/",
		h,
	)

	handler := middleware.Wrap(mux,
		middleware.RateLimiter(serverConfig.GetRateOrDefault()),
		middleware.RequestLogger(logger, []string{"/health", "/ready", "/metrics"}),
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
