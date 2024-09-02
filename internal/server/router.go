package server

import (
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v2/internal/server/handlers"
	"github.com/gorilla/mux"
)

//nolint:funlen // We should describe all api path in one place.
func NewRouter(apiPath, sysPath string, h *handlers.Service, middlewares ...mux.MiddlewareFunc) *mux.Router {
	r := mux.NewRouter()

	// System sub router.
	sysRouter := r.PathPrefix(sysPath).Subrouter()
	// API sub router.
	apiRouter := r.PathPrefix(apiPath).Subrouter()

	// Apply middlewares.
	applyMiddleware(sysRouter, middlewares...)
	applyMiddleware(apiRouter, middlewares...)

	// root route
	sysRouter.HandleFunc("/", handlers.RootActionHandler).Methods(http.MethodGet)

	// health route
	sysRouter.HandleFunc("/health", handlers.HealthActionHandler).Methods(http.MethodGet)

	// readiness route
	sysRouter.HandleFunc("/ready", handlers.ReadyActionHandler).Methods(http.MethodGet)

	// version route
	sysRouter.HandleFunc("/version", handlers.VersionActionHandler).Methods(http.MethodGet)

	// Prometheus endpoint
	sysRouter.Handle("/metrics", handlers.MetricsActionHandler()).Methods(http.MethodGet)

	// OpenAPI specification endpoint
	sysRouter.PathPrefix("/api-docs/").Handler(handlers.APIDocsActionHandler()).Methods(http.MethodGet)

	// whole config route
	apiRouter.HandleFunc("/config", h.ConfigActionHandler).Methods(http.MethodGet, http.MethodPut)
	// apply config after update
	apiRouter.HandleFunc("/config/apply", h.ApplyConfig).Methods(http.MethodPost)

	// cluster config routes
	apiRouter.HandleFunc("/config/clusters/{name}", h.ConfigClusterActionHandler).
		Methods(http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete)
	apiRouter.HandleFunc("/config/clusters", h.ReadAerospikeClusters).Methods(http.MethodGet)

	// storage config routes
	apiRouter.HandleFunc("/config/storage/{name}", h.ConfigStorageActionHandler).
		Methods(http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete)
	apiRouter.HandleFunc("/config/storage", h.ReadAllStorage).Methods(http.MethodGet)

	// policy config routes
	apiRouter.HandleFunc("/config/policies/{name}", h.ConfigPolicyActionHandler).
		Methods(http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete)
	apiRouter.HandleFunc("/config/policies", h.ReadPolicies).Methods(http.MethodGet)

	// routine config routes
	apiRouter.HandleFunc("/config/routines/{name}", h.ConfigRoutineActionHandler).
		Methods(http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete)
	apiRouter.HandleFunc("/config/routines", h.ReadRoutines).Methods(http.MethodGet)

	// Restore job endpoints
	// Restore from full backup (by folder)
	apiRouter.HandleFunc("/restore/full", h.RestoreFullHandler).Methods(http.MethodPost)

	// Restore from incremental backup (by file)
	apiRouter.HandleFunc("/restore/incremental", h.RestoreIncrementalHandler).Methods(http.MethodPost)

	// Restore to specific point in time (by timestamp and routine)
	apiRouter.HandleFunc("/restore/timestamp", h.RestoreByTimeHandler).Methods(http.MethodPost)

	// Restore job status endpoint
	apiRouter.HandleFunc("/restore/status/{jobId}", h.RestoreStatusHandler).Methods(http.MethodGet)

	// Return backed up Aerospike configuration
	apiRouter.HandleFunc("/retrieve/configuration/{name}/{timestamp}", h.RetrieveConfig).Methods(http.MethodGet)

	// Read available backups
	apiRouter.HandleFunc("/backups/full/{name}", h.GetFullBackupsForRoutine).Methods(http.MethodGet)
	apiRouter.HandleFunc("/backups/full", h.GetAllFullBackups).Methods(http.MethodGet)
	apiRouter.HandleFunc("/backups/incremental/{name}", h.GetIncrementalBackupsForRoutine).Methods(http.MethodGet)
	apiRouter.HandleFunc("/backups/incremental", h.GetAllIncrementalBackups).Methods(http.MethodGet)

	// Schedules a full backup operation
	apiRouter.HandleFunc("/backups/schedule/{name}", h.ScheduleFullBackup).Methods(http.MethodPost)

	// Get information on currently running backups
	apiRouter.HandleFunc("/backups/currentBackup/{name}", h.GetCurrentBackupInfo).Methods(http.MethodGet)

	return r
}

func applyMiddleware(router *mux.Router, middlewares ...mux.MiddlewareFunc) {
	for _, m := range middlewares {
		router.Use(m)
	}
}
