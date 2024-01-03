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
	"golang.org/x/time/rate"
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
			} else {
				addresses[ip] = &ipAddr
			}
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
	restoreService service.RestoreService
	backupBackends map[string]service.BackupListReader
}

// Annotations to generate OpenAPI description (https://github.com/swaggo/swag)
// @title           Backup Service REST API Specification
// @version         0.1.0
// @description     Aerospike Backup Service
// @license.name    Apache 2.0
// @license.url     http://www.apache.org/licenses/LICENSE-2.0.html
// @host            localhost:8080
//
// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
//
// NewHTTPServer returns a new instance of HTTPServer.
func NewHTTPServer(handlers []service.BackupScheduler, config *model.Config) *HTTPServer {
	addr := fmt.Sprintf("%s:%d", config.HTTPServer.Address, config.HTTPServer.Port)

	backendMap := make(map[string]service.BackupListReader, len(handlers))
	for _, backend := range handlers {
		backendMap[backend.BackupRoutineName()] = backend.GetBackend()
	}
	rateLimiter := NewIPRateLimiter(
		rate.Limit(config.HTTPServer.Rate.Tps),
		config.HTTPServer.Rate.Size,
	)
	return &HTTPServer{
		config: config,
		server: &http.Server{
			Addr: addr,
		},
		rateLimiter:    rateLimiter,
		whiteList:      newIPWhiteList(config.HTTPServer.Rate.WhiteList),
		restoreService: service.NewRestoreMemory(backendMap, config),
		backupBackends: backendMap,
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
func (ws *HTTPServer) Start() {
	mux := http.NewServeMux()

	// root route
	mux.HandleFunc("/", rootActionHandler)

	// whole config route
	mux.HandleFunc("/config", ws.configActionHandler)

	// cluster config route
	mux.HandleFunc("/config/cluster", ws.configClusterActionHandler)

	// storage config route
	mux.HandleFunc("/config/storage", ws.configStorageActionHandler)

	// policy config route
	mux.HandleFunc("/config/policy", ws.configPolicyActionHandler)

	// routine config route
	mux.HandleFunc("/config/routine", ws.configRoutineActionHandler)

	// health route
	mux.HandleFunc("/health", healthActionHandler)

	// readiness route
	mux.HandleFunc("/ready", readyActionHandler)

	// version route
	mux.HandleFunc("/version", versionActionHandler)

	// Prometheus endpoint
	mux.Handle("/metrics", metricsActionHandler())

	// Restore job endpoints
	// Restore from full backup (by folder)
	mux.HandleFunc("/restore/full", ws.restoreFullHandler)

	// Restore from incremental backup (by file)
	mux.HandleFunc("/restore/incremental", ws.restoreIncrementalHandler)

	// Restore to specific point in time (by timestamp and routine)
	mux.HandleFunc("/restore/timestamp", ws.restoreByTimeHandler)

	// Restore job status endpoint
	mux.HandleFunc("/restore/status", ws.restoreStatusHandler)

	// Returns a list of available full backups for the given policy name
	mux.HandleFunc("/backup/full/list", ws.getAvailableFullBackups)

	// Returns a list of available incremental backups for the given policy name
	mux.HandleFunc("/backup/incremental/list", ws.getAvailableIncrementalBackups)

	ws.server.Handler = ws.rateLimiterMiddleware(mux)
	err := ws.server.ListenAndServe()
	if strings.Contains(err.Error(), "Server closed") {
		slog.Info(err.Error())
	} else {
		panic(err)
	}
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
		ws.readAerospikeClusters(w)
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
		ws.readStorage(w)
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
		ws.readPolicies(w)
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
		ws.readRoutines(w)
	case http.MethodPut:
		ws.updateRoutine(w, r)
	case http.MethodDelete:
		ws.deleteRoutine(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
