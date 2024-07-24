package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/reugn/go-quartz/quartz"
	"golang.org/x/time/rate"
)

const (
	restAPIVersion = "v1"
)

type ipWhiteList struct {
	addresses map[string]*netip.Addr
	networks  []*netip.Prefix
	allowAny  bool
}

func newIPWhiteList(ipList []string) *ipWhiteList {
	addresses := make(map[string]*netip.Addr)
	networks := make([]*netip.Prefix, 0)
	var allowAny bool
	for _, ip := range ipList {
		if strings.HasPrefix(ip, "0.0.0.0") {
			allowAny = true
		}
		network, err := netip.ParsePrefix(ip)
		if err != nil {
			ipAddr, err := netip.ParseAddr(ip)
			if err != nil {
				panic(fmt.Sprintf("invalid ip configuration: %s", ip))
			}
			addresses[ip] = &ipAddr
		} else {
			networks = append(networks, &network)
		}
	}
	return &ipWhiteList{
		addresses: addresses,
		networks:  networks,
		allowAny:  allowAny,
	}
}

func (wl *ipWhiteList) isAllowed(ip string) bool {
	if wl.allowAny {
		return true
	}
	ipAddr, err := netip.ParseAddr(ip)
	if err != nil {
		slog.Warn("Invalid client ip", "ip", ip)
		return false
	}
	_, ok := wl.addresses[ip]
	if ok {
		return true
	}

	for _, network := range wl.networks {
		if network.Contains(ipAddr) {
			return true
		}
	}

	return false
}

// HTTPServer is the backup service HTTP server wrapper.
type HTTPServer struct {
	config         *model.Config
	server         *http.Server
	rateLimiter    *IPRateLimiter
	whiteList      *ipWhiteList
	scheduler      quartz.Scheduler
	restoreService service.RestoreService
	backupBackends service.BackendsHolder
	handlerHolder  *service.BackupHandlerHolder
}

// NewHTTPServer returns a new instance of HTTPServer.
func NewHTTPServer(
	backends service.BackendsHolder,
	config *model.Config,
	scheduler quartz.Scheduler,
	handlerHolder *service.BackupHandlerHolder,
) *HTTPServer {
	serverConfig := config.ServiceConfig.HTTPServer
	addr := fmt.Sprintf("%s:%d", serverConfig.GetAddressOrDefault(), serverConfig.GetPortOrDefault())

	rateLimiter := NewIPRateLimiter(
		rate.Limit(serverConfig.GetRateOrDefault().GetTpsOrDefault()),
		serverConfig.GetRateOrDefault().GetSizeOrDefault(),
	)
	return &HTTPServer{
		config: config,
		server: &http.Server{
			Addr: addr,
		},
		rateLimiter:    rateLimiter,
		whiteList:      newIPWhiteList(serverConfig.GetRateOrDefault().GetWhiteListOrDefault()),
		scheduler:      scheduler,
		restoreService: service.NewRestoreMemory(backends, config, shared.NewRestoreGo()),
		backupBackends: backends,
		handlerHolder:  handlerHolder,
	}
}

func (ws *HTTPServer) rateLimiterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if !ws.whiteList.isAllowed(ip) {
			limiter := ws.rateLimiter.GetLimiter(ip)
			if !limiter.Allow() {
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// Start starts the HTTP server.
//
//nolint:funlen
func (ws *HTTPServer) Start() {
	mux := http.NewServeMux()

	// root route
	mux.HandleFunc(ws.sys("/"), rootActionHandler)

	// whole config route
	mux.HandleFunc(ws.api("/config"), ws.configActionHandler)
	// apply config after update
	mux.HandleFunc(ws.api("/config/apply"), ws.applyConfig)

	// cluster config routes
	mux.HandleFunc(ws.api("/config/clusters/{name}"), ws.configClusterActionHandler)
	mux.HandleFunc(ws.api("/config/clusters"), ws.readAerospikeClusters)

	// storage config routes
	mux.HandleFunc(ws.api("/config/storage/{name}"), ws.configStorageActionHandler)
	mux.HandleFunc(ws.api("/config/storage"), ws.readAllStorage)

	// policy config routes
	mux.HandleFunc(ws.api("/config/policies/{name}"), ws.configPolicyActionHandler)
	mux.HandleFunc(ws.api("/config/policies"), ws.readPolicies)

	// routine config routes
	mux.HandleFunc(ws.api("/config/routines/{name}"), ws.configRoutineActionHandler)
	mux.HandleFunc(ws.api("/config/routines"), ws.readRoutines)

	// health route
	mux.HandleFunc(ws.sys("/health"), healthActionHandler)

	// readiness route
	mux.HandleFunc(ws.sys("/ready"), readyActionHandler)

	// version route
	mux.HandleFunc(ws.sys("/version"), versionActionHandler)

	// Prometheus endpoint
	mux.Handle(ws.sys("/metrics"), metricsActionHandler())

	// OpenAPI specification endpoint
	mux.Handle(ws.sys("/api-docs/"), apiDocsActionHandler())

	// Restore job endpoints
	// Restore from full backup (by folder)
	mux.HandleFunc(ws.api("/restore/full"), ws.restoreFullHandler)

	// Restore from incremental backup (by file)
	mux.HandleFunc(ws.api("/restore/incremental"), ws.restoreIncrementalHandler)

	// Restore to specific point in time (by timestamp and routine)
	mux.HandleFunc(ws.api("/restore/timestamp"), ws.restoreByTimeHandler)

	// Restore job status endpoint
	mux.HandleFunc(ws.api("/restore/status/{jobId}"), ws.restoreStatusHandler)

	// Return backed up Aerospike configuration
	mux.HandleFunc(ws.api("/retrieve/configuration/{name}/{timestamp}"), ws.retrieveConfig)

	// Read available backups
	mux.HandleFunc(ws.api("/backups/full/{name}"), ws.getFullBackupsForRoutine)
	mux.HandleFunc(ws.api("/backups/full"), ws.getAllFullBackups)
	mux.HandleFunc(ws.api("/backups/incremental/{name}"), ws.getIncrementalBackupsForRoutine)
	mux.HandleFunc(ws.api("/backups/incremental"), ws.getAllIncrementalBackups)

	// Schedules a full backup operation
	mux.HandleFunc(ws.api("/backups/schedule/{name}"), ws.scheduleFullBackup)

	mux.HandleFunc(ws.api("/backups/currentBackup/{name}"), ws.getCurrentBackupInfo)

	ws.server.Handler = ws.rateLimiterMiddleware(mux)
	err := ws.server.ListenAndServe()
	if err != nil && strings.Contains(err.Error(), "Server closed") {
		slog.Info(err.Error())
	} else {
		panic(err)
	}
}

func (ws *HTTPServer) api(pattern string) string {
	contextPath := ws.config.ServiceConfig.HTTPServer.GetContextPathOrDefault()
	if !strings.HasSuffix(contextPath, "/") {
		contextPath += "/"
	}
	return fmt.Sprintf("%s%s%s", contextPath, restAPIVersion, pattern)
}

func (ws *HTTPServer) sys(pattern string) string {
	contextPath := ws.config.ServiceConfig.HTTPServer.GetContextPathOrDefault()
	if contextPath == "/" {
		return pattern
	}
	if !strings.HasSuffix(contextPath, "/") {
		contextPath += "/"
	}
	return fmt.Sprintf("%s%s", contextPath, pattern)
}

// Shutdown shutdowns the HTTP server.
func (ws *HTTPServer) Shutdown() error {
	return ws.server.Shutdown(context.Background())
}

func (ws *HTTPServer) configActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ws.readConfig(w)
	case http.MethodPut:
		ws.updateConfig(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ws *HTTPServer) configClusterActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		ws.addAerospikeCluster(w, r)
	case http.MethodGet:
		ws.readAerospikeCluster(w, r)
	case http.MethodPut:
		ws.updateAerospikeCluster(w, r)
	case http.MethodDelete:
		ws.deleteAerospikeCluster(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ws *HTTPServer) configStorageActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		ws.addStorage(w, r)
	case http.MethodGet:
		ws.readStorage(w, r)
	case http.MethodPut:
		ws.updateStorage(w, r)
	case http.MethodDelete:
		ws.deleteStorage(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ws *HTTPServer) configPolicyActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		ws.addPolicy(w, r)
	case http.MethodGet:
		ws.readPolicy(w, r)
	case http.MethodPut:
		ws.updatePolicy(w, r)
	case http.MethodDelete:
		ws.deletePolicy(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ws *HTTPServer) configRoutineActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		ws.addRoutine(w, r)
	case http.MethodGet:
		ws.readRoutine(w, r)
	case http.MethodPut:
		ws.updateRoutine(w, r)
	case http.MethodDelete:
		ws.deleteRoutine(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
