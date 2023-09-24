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
}

// NewHTTPServer returns a new instance of HTTPServer.
func NewHTTPServer(host string, port int, config *model.Config) *HTTPServer {
	addr := host + ":" + strconv.Itoa(port)

	return &HTTPServer{
		config: config,
		server: &http.Server{
			Addr: addr,
		},
		restoreService: service.NewRestoreMemory(),
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

	ws.server.Handler = rateLimiterMiddleware(mux)
	err := ws.server.ListenAndServe()
	util.Check(err)
}

// Shutdown shutdowns the HTTP server.
func (ws *HTTPServer) Shutdown() error {
	return ws.server.Shutdown(context.Background())
}

func rootActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
	}
	fmt.Fprintf(w, "")
}

func (ws *HTTPServer) configActionHandler(w http.ResponseWriter, _ *http.Request) {
	configuration, err := json.MarshalIndent(ws.config, "", "    ") // pretty print
	if err != nil {
		http.Error(w, "Failed to parse service configuration", http.StatusInternalServerError)
	}
	fmt.Fprint(w, string(configuration))
}

func healthActionHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "Ok")
}

func readyActionHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "Ok")
}

func versionActionHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprint(w, util.Version)
}

func (ws *HTTPServer) restoreHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var request model.RestoreRequest

		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
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

func (ws *HTTPServer) restoreStatusHandler(w http.ResponseWriter, r *http.Request) {
	jobIDParam := r.URL.Query().Get("jobId")
	jobID, err := strconv.Atoi(jobIDParam)
	if err != nil {
		http.Error(w, "Invalid job id", http.StatusBadRequest)
	} else {
		fmt.Fprint(w, ws.restoreService.JobStatus(jobID))
	}
}
