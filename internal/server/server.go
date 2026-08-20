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
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

const (
	restAPIVersion  = "v1"
	shutdownTimeout = 30 * time.Second
)

// HTTPServer manages the backup service HTTP server lifecycle.
type HTTPServer interface {
	http.Handler

	// Start starts the HTTP server. Returns an error if the server fails to start.
	Start() error
	// Shutdown shuts down the HTTP server gracefully with a timeout.
	Shutdown() error
}

// httpServer wraps *http.Server with Start/Shutdown lifecycle helpers.
type httpServer struct {
	*http.Server
}

var _ HTTPServer = (*httpServer)(nil)

// NewHTTPServer returns a new instance of HTTPServer.
func NewHTTPServer(ctx context.Context, serverConfig *model.HTTPServerConfig, service *handlers.Service) HTTPServer {
	addr := fmt.Sprintf("%s:%d", serverConfig.GetAddressOrDefault(), serverConfig.GetPortOrDefault())

	// Create router
	mux := NewServeMux(
		"/"+restAPIVersion,
		"/",
		service,
	)

	handler := middleware.Wrap(mux,
		middleware.RequestLogger(slog.Default(), []string{"health", "ready", "metrics"}),
		middleware.RateLimiter(ctx, serverConfig.GetRateOrDefault()),
	)

	return &httpServer{
		Server: &http.Server{
			Addr:              addr,
			ReadHeaderTimeout: serverConfig.GetTimeoutOrDefault(),
			ReadTimeout:       serverConfig.GetReadTimeoutOrDefault(),
			WriteTimeout:      serverConfig.GetWriteTimeoutOrDefault(),
			IdleTimeout:       serverConfig.GetIdleTimeoutOrDefault(),
			Handler:           handler,
		},
	}
}

// ServeHTTP implements http.Handler so integration tests can wrap the server with httptest.
func (s *httpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Handler.ServeHTTP(w, r)
}

// Start starts the HTTP server. Returns an error if the server fails to start.
func (s *httpServer) Start() error {
	err := s.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		slog.Info("HTTP server closed", attr.Error(err))
		return nil
	}
	return err
}

// Shutdown shuts down the HTTP server gracefully with a timeout.
func (s *httpServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return s.Server.Shutdown(ctx)
}
