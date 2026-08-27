package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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
}

var _ HTTP = (*serverHTTP)(nil)

// NewServerHTTP returns a new instance of HTTP.
func NewServerHTTP(ctx context.Context, serverConfig *model.ServerConfigHTTP, service *handlers.Service) HTTP {
	return newServerHTTP(ctx,
		&serverConfig.ListenerConfig,
		fmt.Sprintf("%s:%d", serverConfig.GetAddressOrDefault(), serverConfig.GetPortOrDefault()),
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
		&serverConfig.ListenerConfig,
		fmt.Sprintf("%s:%d", serverConfig.GetAddressOrDefault(), serverConfig.GetPortOrDefault()),
		service,
		tlsConfig,
	), nil
}

func newServerHTTP(
	ctx context.Context,
	listener *model.ListenerConfig,
	addr string,
	service *handlers.Service,
	tlsConfig *tls.Config,
) HTTP {
	sysPath := normalizeContextPath(listener.GetContextPathOrDefault())
	mux := NewServeMux(
		sysPath+restAPIVersion,
		sysPath,
		service,
	)

	handler := middleware.Wrap(mux,
		middleware.RequestLogger(slog.Default(), []string{"health", "ready", "metrics"}),
		middleware.RateLimiter(ctx, listener.GetRateOrDefault()),
	)

	return &serverHTTP{
		Server: &http.Server{
			Addr:              addr,
			ReadHeaderTimeout: listener.GetTimeoutOrDefault(),
			ReadTimeout:       listener.GetReadTimeoutOrDefault(),
			WriteTimeout:      listener.GetWriteTimeoutOrDefault(),
			IdleTimeout:       listener.GetIdleTimeoutOrDefault(),
			Handler:           handler,
			TLSConfig:         tlsConfig,
		},
	}
}

// normalizeContextPath returns the context path bounded by slashes so route
// patterns can be appended to it directly.
func normalizeContextPath(contextPath string) string {
	if !strings.HasPrefix(contextPath, "/") {
		contextPath = "/" + contextPath
	}
	if !strings.HasSuffix(contextPath, "/") {
		contextPath += "/"
	}

	return contextPath
}

// ServeHTTP implements http.Handler so integration tests can wrap the server with httptest.
func (s *serverHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Handler.ServeHTTP(w, r)
}

// Start starts the HTTP server. Returns an error if the server fails to start.
func (s *serverHTTP) Start() error {
	if s.TLSConfig == nil {
		return s.startHTTP()
	}

	// The key pair is already loaded into TLSConfig, so no certificate files are
	// passed here. Serving through net/http also negotiates HTTP/2.
	err := s.ListenAndServeTLS("", "")
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

// Run starts the given HTTP/HTTPS servers concurrently and blocks until the context is canceled
// or one of the servers encounters an error, then shuts all servers down gracefully.
func Run(ctx context.Context, servers []HTTP) error {
	errCh := make(chan error, len(servers))
	for _, srv := range servers {
		go func() {
			errCh <- srv.Start()
		}()
	}

	// Wait for either context cancellation or server error
	select {
	case err := <-errCh:
		if err != nil {
			for _, srv := range servers {
				_ = srv.Shutdown()
			}
			for range len(servers) - 1 {
				<-errCh
			}
			return fmt.Errorf("HTTP server failed: %w", err)
		}
	case <-ctx.Done():
	}

	var shutdownErr error
	for _, srv := range servers {
		if err := srv.Shutdown(); err != nil {
			slog.Error("HTTP server shutdown failed", attr.Error(err))
			if shutdownErr == nil {
				shutdownErr = err
			}
		}
	}
	for range len(servers) {
		<-errCh
	}
	if shutdownErr != nil {
		return shutdownErr
	}

	slog.Info("HTTP server shut down gracefully")

	return nil
}
