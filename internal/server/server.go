package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

	// status route
	mux.HandleFunc("/config", ws.configActionHandler)

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

// @Summary      Root endpoint
// @Router       / [get]
// @Success 200 ""
func rootActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
	}
	fmt.Fprintf(w, "")
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

// @Summary      Health endpoint.
// @Router       /health [get]
// @Success 200  "Ok"
func healthActionHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "Ok")
}

// @Summary      Readiness endpoint.
// @Router       /ready [get]
// @Success 200  "Ok"
func readyActionHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "Ok")
}

// @Summary      Returns application version.
// @Router       /version [get]
// @Success 200 {string} string "version"
func versionActionHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprint(w, util.Version)
}

// @Summary      Trigger an asynchronous restore operation.
// @Description  Specify the directory parameter for the full backup restore.
// @Description  Use the file parameter to restore from an incremental backup file.
// @Router       /restore [post]
// @Param request body model.RestoreRequest true "query params"
// @Success 200 {integer} int "Job ID (int64)"
func (ws *HTTPServer) restoreHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var request model.RestoreRequest

		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err = request.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		jobID := ws.restoreService.Restore(&request)
		slog.Info("Restore action", "jobID", jobID, "request", request)
		fmt.Fprint(w, strconv.Itoa(jobID))
	} else {
		http.Error(w, "", http.StatusNotFound)
	}
}

// @Summary 	Retrieve status for a restore job.
// @Produce plain
// @Param jobId query int true "Job ID to retrieve the status"
// @Router /restore/status [get]
// @Success 200 {string} string "Job status"
func (ws *HTTPServer) restoreStatusHandler(w http.ResponseWriter, r *http.Request) {
	jobIDParam := r.URL.Query().Get("jobId")
	jobID, err := strconv.Atoi(jobIDParam)
	if err != nil {
		http.Error(w, "Invalid job id", http.StatusBadRequest)
	} else {
		fmt.Fprint(w, ws.restoreService.JobStatus(jobID))
	}
}

// @Summary 	Get available full backups.
// @Produce plain
// @Param name query string true "Backup policy name"
// @Router /backup/full/list [get]
// @Success 200 {array} model.BackupDetails "Full backups"
// @Failure 404 {string} string ""
func (ws *HTTPServer) getAvailableFullBackups(w http.ResponseWriter, r *http.Request) {
	policyName := r.URL.Query().Get("name")
	if policyName == "" {
		http.Error(w, "Invalid/undefined policy name", http.StatusBadRequest)
	} else {
		list, err := ws.backupBackends[policyName].FullBackupList()
		if err != nil {
			slog.Error("Get full backup list", "err", err)
			http.Error(w, "", http.StatusNotFound)
		} else {
			response, err := json.Marshal(list)
			if err != nil {
				slog.Error("Failed to parse full backup list", "err", err)
				http.Error(w, "", http.StatusInternalServerError)
			} else {
				fmt.Fprint(w, string(response))
			}
		}
	}
}

// @Summary 	Get available incremental backups.
// @Produce plain
// @Param name query string true "Backup policy name"
// @Router /backup/incremental/list [get]
// @Success 200 {array} model.BackupDetails "Incremental backups"
// @Failure 404 {string} string ""
func (ws *HTTPServer) getAvailableIncrBackups(w http.ResponseWriter, r *http.Request) {
	policyName := r.URL.Query().Get("name")
	if policyName == "" {
		http.Error(w, "Invalid/undefined policy name", http.StatusBadRequest)
	} else {
		list, err := ws.backupBackends[policyName].IncrementalBackupList()
		if err != nil {
			slog.Error("Get incremental backup list", "err", err)
			http.Error(w, "", http.StatusNotFound)
		} else {
			response, err := json.Marshal(list)
			if err != nil {
				slog.Error("Failed to parse incremental backup list", "err", err)
				http.Error(w, "", http.StatusInternalServerError)
			} else {
				fmt.Fprint(w, string(response))
			}
		}
	}
}
