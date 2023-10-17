package server

import (
	"context"
	"net"
	"net/http"
	"strconv"

	"github.com/aerospike/backup/internal/util"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var rateLimiter = NewIPRateLimiter(1, 10)

var ipsWhiteList = map[string]struct{}{
	"127.0.0.1": {},
}

// HTTPServer is the authentication HTTP server wrapper.
type HTTPServer struct {
	config         *model.Config
	server         *http.Server
	restoreService service.RestoreService
	backupBackends map[string]service.BackupBackend
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
func NewHTTPServer(host string, port int, backends []service.BackupBackend,
	config *model.Config) *HTTPServer {
	addr := host + ":" + strconv.Itoa(port)

	backendMap := make(map[string]service.BackupBackend, len(backends))
	for _, backend := range backends {
		backendMap[backend.BackupPolicyName()] = backend
	}
	return &HTTPServer{
		config: config,
		server: &http.Server{
			Addr: addr,
		},
		restoreService: service.NewRestoreMemory(),
		backupBackends: backendMap,
	}
}

func rateLimiterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		_, ok := ipsWhiteList[ip]
		if !ok {
			limiter := rateLimiter.GetLimiter(ip)
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

	// health route
	mux.HandleFunc("/health", healthActionHandler)

	// readiness route
	mux.HandleFunc("/ready", readyActionHandler)

	// version route
	mux.HandleFunc("/version", versionActionHandler)

	// Prometheus endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Restore job endpoint
	mux.HandleFunc("/restore", ws.restoreHandler)

	// Restore job status endpoint
	mux.HandleFunc("/restore/status", ws.restoreStatusHandler)

	// Returns a list of available full backups for the given policy name
	mux.HandleFunc("/backup/full/list", ws.getAvailableFullBackups)

	// Returns a list of available incremental backups for the given policy name
	mux.HandleFunc("/backup/incremental/list", ws.getAvailableIncrBackups)

	ws.server.Handler = rateLimiterMiddleware(mux)
	err := ws.server.ListenAndServe()
	util.Check(err)
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ws *HTTPServer) configStorageActionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		ws.addStorage(w, r)
	case http.MethodGet:
		ws.readStorages(w)
	case http.MethodPut:
		ws.updateStorage(w, r)
	case http.MethodDelete:
		ws.deleteStorage(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
