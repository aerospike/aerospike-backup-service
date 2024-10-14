package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v2/internal/server/middleware"
	"github.com/aerospike/aerospike-backup-service/v2/internal/util"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
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
func NewHTTPServer(serverConfig *model.HTTPServerConfig, h *handlers.Service) *HTTPServer {
	addr := fmt.Sprintf("%s:%d", serverConfig.GetAddressOrDefault(), serverConfig.GetPortOrDefault())
	httpTimeout := time.Duration(serverConfig.GetTimeout()) * time.Millisecond

	rateLimiter := util.NewIPRateLimiter(
		rate.Limit(serverConfig.GetRateOrDefault().GetTpsOrDefault()),
		serverConfig.GetRateOrDefault().GetSizeOrDefault(),
	)

	whitelist := util.NewIPWhiteList(serverConfig.GetRateOrDefault().GetWhiteListOrDefault())
	mw := middleware.RateLimiter(rateLimiter, whitelist)

	router := NewRouter(
		fmt.Sprintf("/%s", restAPIVersion),
		"/",
		h,
		mw)

	return &HTTPServer{
		server: &http.Server{
			Addr:              addr,
			ReadHeaderTimeout: httpTimeout,
			Handler:           router,
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
