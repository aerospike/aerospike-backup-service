package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/middleware"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

const (
	restAPIVersion  = "v1"
	shutdownTimeout = 30 * time.Second
)

// HTTPServer is the backup service HTTP server wrapper.
type HTTPServer struct {
	server *http.Server
}

// NewHTTPServer returns a new instance of HTTPServer.
func NewHTTPServer(serverConfig *model.HTTPServerConfig, service *handlers.Service, logger *slog.Logger) *HTTPServer {
	addr := fmt.Sprintf("%s:%d", serverConfig.GetAddressOrDefault(), serverConfig.GetPortOrDefault())

	// Create router
	mux := NewServeMux(
		fmt.Sprintf("/%s", restAPIVersion),
		"/",
		service,
	)

	handler := middleware.Wrap(mux,
		middleware.RateLimiter(serverConfig.GetRateOrDefault()),
		middleware.RequestLogger(logger, []string{"health", "ready", "metrics"}),
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
		slog.Info(err.Error())
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
