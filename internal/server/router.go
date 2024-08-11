package server

import (
	"net/http"

	"github.com/aerospike/backup/internal/server/handlers"
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
	sysRouter.HandleFunc("/", handlers.RootActionHandler)

	// health route
	sysRouter.HandleFunc("/health", handlers.HealthActionHandler)

	// readiness route
	sysRouter.HandleFunc("/ready", handlers.ReadyActionHandler)

	// version route
	sysRouter.HandleFunc("/version", handlers.VersionActionHandler)

	// Prometheus endpoint
	sysRouter.Handle("/metrics", handlers.MetricsActionHandler())

	// OpenAPI specification endpoint
	sysRouter.Handle("/api-docs/", handlers.APIDocsActionHandler())

	// whole config route
	apiRouter.HandleFunc("/config", h.ConfigActionHandler).Methods(http.MethodGet, http.MethodPut)
	// apply config after update
	apiRouter.HandleFunc("/config/apply", h.ApplyConfig).Methods(http.MethodPost)

	// cluster config routes
	apiRouter.HandleFunc("/config/clusters/{name}", h.ConfigClusterActionHandler).
		Methods(http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete)
	apiRouter.HandleFunc("/config/clusters", h.ReadAerospikeClusters)

	// storage config routes
	apiRouter.HandleFunc("/config/storage/{name}", h.ConfigStorageActionHandler)
	apiRouter.HandleFunc("/config/storage", h.ReadAllStorage)

	// policy config routes
	apiRouter.HandleFunc("/config/policies/{name}", h.ConfigPolicyActionHandler).
		Methods(http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete)
	apiRouter.HandleFunc("/config/policies", h.ReadPolicies)

	// routine config routes
	apiRouter.HandleFunc("/config/routines/{name}", h.ConfigRoutineActionHandler)
	apiRouter.HandleFunc("/config/routines", h.ReadRoutines)

	// Restore job endpoints
	// Restore from full backup (by folder)
	apiRouter.HandleFunc("/restore/full", h.RestoreFullHandler)

	// Restore from incremental backup (by file)
	apiRouter.HandleFunc("/restore/incremental", h.RestoreIncrementalHandler)

	// Restore to specific point in time (by timestamp and routine)
	apiRouter.HandleFunc("/restore/timestamp", h.RestoreByTimeHandler)

	// Restore job status endpoint
	apiRouter.HandleFunc("/restore/status/{jobId}", h.RestoreStatusHandler)

	// Return backed up Aerospike configuration
	apiRouter.HandleFunc("/retrieve/configuration/{name}/{timestamp}", h.RetrieveConfig)

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
