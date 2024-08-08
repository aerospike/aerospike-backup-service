package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/aerospike/backup/internal/server/handlers"
	"github.com/aerospike/backup/internal/server/middleware"
	"github.com/aerospike/backup/internal/util"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"github.com/reugn/go-quartz/quartz"
	"golang.org/x/time/rate"
)

const (
	restAPIVersion = "v1"
)

// HTTPServer is the backup service HTTP server wrapper.
type HTTPServer struct {
	config *model.Config
	server *http.Server
}

// NewHTTPServer returns a new instance of HTTPServer.
func NewHTTPServer(
	config *model.Config,
	scheduler quartz.Scheduler,
	backends service.BackendsHolder,
	handlerHolder service.BackupHandlerHolder,
	configurationManager service.ConfigurationManager,
	logger *slog.Logger,
) *HTTPServer {
	serverConfig := config.ServiceConfig.HTTPServer

	addr := fmt.Sprintf("%s:%d", serverConfig.GetAddressOrDefault(), serverConfig.GetPortOrDefault())

	rateLimiter := util.NewIPRateLimiter(
		rate.Limit(serverConfig.GetRateOrDefault().GetTpsOrDefault()),
		serverConfig.GetRateOrDefault().GetSizeOrDefault(),
	)

	whitelist := util.NewIPWhiteList(serverConfig.GetRateOrDefault().GetWhiteListOrDefault())

	mw := middleware.RateLimiter(rateLimiter, whitelist)

	restoreMgr := service.NewRestoreManager(backends, config, service.NewRestoreGo())

	h := handlers.NewService(config, scheduler, restoreMgr, backends, handlerHolder, configurationManager, logger)

	router := NewRouter(
		fmt.Sprintf("/%s", restAPIVersion),
		"/",
		h,
		mw)

	return &HTTPServer{
		config: config,
		server: &http.Server{
			Addr:              addr,
			ReadHeaderTimeout: 5 * time.Second,
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
