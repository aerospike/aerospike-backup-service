package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/middleware"
)

const (
	restAPIVersion  = "v1"
	shutdownTimeout = 30 * time.Second
)

// Server represents an HTTP server that can be started and gracefully shut down.
type Server interface {
	// Start starts the server.
	Start() error
	// Shutdown gracefully shuts down the server.
	Shutdown() error
}

// HTTPServer is the backup service HTTP server wrapper.
type HTTPServer struct {
	server *http.Server
}

// NewHTTPServer returns a new instance of HTTPServer.
func NewHTTPServer(service *handlers.Service) Server {
	serverConfig := service.HTTPServerConfig()
	addr := fmt.Sprintf("%s:%d", serverConfig.GetAddressOrDefault(), serverConfig.GetPortOrDefault())

	// Create router
	mux := NewServeMux(
		"/"+restAPIVersion,
		"/",
		service,
	)

	handler := middleware.Wrap(mux,
		middleware.RateLimiter(serverConfig.GetRateOrDefault()),
		middleware.RequestLogger(slog.Default(), []string{"health", "ready", "metrics"}),
	)

	return &HTTPServer{
		server: &http.Server{
			Addr:              addr,
			ReadHeaderTimeout: serverConfig.GetTimeoutOrDefault(),
			Handler:           handler,
		},
	}
}

// Start starts the HTTP server. Returns an error if the server fails to start.
func (s *HTTPServer) Start() error {
	err := s.server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		slog.Info("HTTP server closed", attr.Error(err))
		return nil
	}
	return err
}

// Shutdown shuts down the HTTP server gracefully with a timeout.
func (s *HTTPServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return s.server.Shutdown(ctx)
}
