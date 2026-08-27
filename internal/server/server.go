package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/middleware"
	servertls "github.com/aerospike/aerospike-backup-service/v3/internal/server/tlsconfig"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
)

const (
	restAPIVersion  = "v1"
	shutdownTimeout = 30 * time.Second
)

// HTTP manages the backup service HTTP server lifecycle.
type HTTP interface {
	http.Handler

	// Start starts the HTTP server. Returns an error if the server fails to start.
	Start() error
	// Shutdown shuts down the HTTP server gracefully with a timeout.
	Shutdown() error
}

// serverHTTP wraps *http.Server with Start/Shutdown lifecycle helpers.
type serverHTTP struct {
	*http.Server
	tlsConfig *tls.Config
}

var _ HTTP = (*serverHTTP)(nil)

// NewServerHTTP returns a new instance of HTTP.
func NewServerHTTP(ctx context.Context, serverConfig *model.ServerConfigHTTP, service *handlers.Service) HTTP {
	return newServerHTTP(ctx,
		fmt.Sprintf("%s:%d", serverConfig.GetAddressOrDefault(), serverConfig.GetPortOrDefault()),
		serverConfig.GetRateOrDefault(),
		serverConfig.GetTimeoutOrDefault(),
		serverConfig.GetReadTimeoutOrDefault(),
		serverConfig.GetWriteTimeoutOrDefault(),
		serverConfig.GetIdleTimeoutOrDefault(),
		service,
		nil,
	)
}

// NewServerHTTPS returns a new instance of an HTTPS server.
func NewServerHTTPS(
	ctx context.Context,
	serverConfig *model.ServerConfigHTTPS,
	service *handlers.Service,
	resolver secrets.Resolver,
) (HTTP, error) {
	tlsConfig, err := servertls.NewTLSConfig(ctx, serverConfig, resolver)
	if err != nil {
		return nil, err
	}

	return newServerHTTP(ctx,
		fmt.Sprintf("%s:%d", serverConfig.GetAddressOrDefault(), serverConfig.GetPortOrDefault()),
		serverConfig.GetRateOrDefault(),
		serverConfig.GetTimeoutOrDefault(),
		serverConfig.GetReadTimeoutOrDefault(),
		serverConfig.GetWriteTimeoutOrDefault(),
		serverConfig.GetIdleTimeoutOrDefault(),
		service,
		tlsConfig,
	), nil
}

func newServerHTTP(
	ctx context.Context,
	addr string,
	rate *model.RateLimiterConfig,
	readHeaderTimeout time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
	idleTimeout time.Duration,
	service *handlers.Service,
	tlsConfig *tls.Config,
) HTTP {
	// Create router
	mux := NewServeMux(
		"/"+restAPIVersion,
		"/",
		service,
	)

	handler := middleware.Wrap(mux,
		middleware.RequestLogger(slog.Default(), []string{"health", "ready", "metrics"}),
		middleware.RateLimiter(ctx, rate),
	)

	return &serverHTTP{
		Server: &http.Server{
			Addr:              addr,
			ReadHeaderTimeout: readHeaderTimeout,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       idleTimeout,
			Handler:           handler,
		},
		tlsConfig: tlsConfig,
	}
}

// ServeHTTP implements http.Handler so integration tests can wrap the server with httptest.
func (s *serverHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Handler.ServeHTTP(w, r)
}

// Start starts the HTTP server. Returns an error if the server fails to start.
func (s *serverHTTP) Start() error {
	if s.tlsConfig == nil {
		return s.startHTTP()
	}

	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", s.Addr)
	if err != nil {
		return err
	}
	tlsListener := tls.NewListener(listener, s.tlsConfig)
	err = s.Serve(tlsListener)
	if errors.Is(err, http.ErrServerClosed) {
		slog.Info("HTTPS server closed", attr.Error(err))
		return nil
	}

	return err
}

func (s *serverHTTP) startHTTP() error {
	err := s.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		slog.Info("HTTP server closed", attr.Error(err))
		return nil
	}
	return err
}

// Shutdown shuts down the HTTP server gracefully with a timeout.
func (s *serverHTTP) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return s.Server.Shutdown(ctx)
}

// Run starts all non-nil HTTP/HTTPS servers concurrently and blocks until the context is canceled
// or one of the servers encounters an error, then shuts all servers down gracefully.
func Run(ctx context.Context, servers ...HTTP) error {
	activeServers := make([]HTTP, 0, len(servers))
	for _, srv := range servers {
		if srv != nil {
			activeServers = append(activeServers, srv)
		}
	}
	if len(activeServers) == 0 {
		return errors.New("no HTTP servers configured")
	}

	// Channel to capture server startup errors
	errCh := make(chan error, len(activeServers))
	for _, srv := range activeServers {
		go func() {
			errCh <- srv.Start()
		}()
	}

	// Wait for either context cancellation or server error
	select {
	case err := <-errCh:
		if err != nil {
			for _, srv := range activeServers {
				_ = srv.Shutdown()
			}
			for range len(activeServers) - 1 {
				<-errCh
			}
			return fmt.Errorf("HTTP server failed: %w", err)
		}
	case <-ctx.Done():
	}

	var shutdownErr error
	for _, srv := range activeServers {
		if err := srv.Shutdown(); err != nil {
			slog.Error("HTTP server shutdown failed", attr.Error(err))
			if shutdownErr == nil {
				shutdownErr = err
			}
		}
	}
	for range len(activeServers) {
		<-errCh
	}
	if shutdownErr != nil {
		return shutdownErr
	}

	slog.Info("HTTP server shut down gracefully")

	return nil
}
