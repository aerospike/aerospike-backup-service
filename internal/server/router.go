package server

import (
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
)

func NewServeMux(apiPath, sysPath string, h *handlers.Service) *http.ServeMux {
	mux := http.NewServeMux()

	registerSystemRoutes(mux, sysPath)
	registerConfigRoutes(mux, apiPath, h)
	registerClusterRoutes(mux, apiPath, h)
	registerStorageRoutes(mux, apiPath, h)
	registerPolicyRoutes(mux, apiPath, h)
	registerRoutineRoutes(mux, apiPath, h)
	registerBackupRoutes(mux, apiPath, h)
	registerRestoreRoutes(mux, apiPath, h)

	return mux
}

func registerSystemRoutes(mux *http.ServeMux, sysPath string) {
	mux.HandleFunc("GET "+sysPath, handlers.RootActionHandler)
	mux.HandleFunc("GET "+sysPath+"health", handlers.HealthActionHandler)
	mux.HandleFunc("GET "+sysPath+"ready", handlers.ReadyActionHandler)
	mux.HandleFunc("GET "+sysPath+"version", handlers.VersionActionHandler)
	mux.Handle("GET "+sysPath+"metrics", handlers.MetricsActionHandler())
	mux.Handle("GET "+sysPath+"api-docs/...", handlers.APIDocsActionHandler())
}

func registerConfigRoutes(mux *http.ServeMux, apiPath string, h *handlers.Service) {
	mux.HandleFunc("GET "+apiPath+"/config", h.ReadConfig)
	mux.HandleFunc("PUT "+apiPath+"/config", h.UpdateConfig)
	mux.HandleFunc("POST "+apiPath+"/config/apply", h.ApplyConfig)
}

func registerClusterRoutes(mux *http.ServeMux, apiPath string, h *handlers.Service) {
	mux.HandleFunc("GET "+apiPath+"/config/clusters", h.ReadAerospikeClusters)
	mux.HandleFunc("POST "+apiPath+"/config/clusters/{name}", h.AddAerospikeCluster)
	mux.HandleFunc("GET "+apiPath+"/config/clusters/{name}", h.ReadAerospikeCluster)
	mux.HandleFunc("PUT "+apiPath+"/config/clusters/{name}", h.UpdateAerospikeCluster)
	mux.HandleFunc("DELETE "+apiPath+"/config/clusters/{name}", h.DeleteAerospikeCluster)
}

func registerStorageRoutes(mux *http.ServeMux, apiPath string, h *handlers.Service) {
	mux.HandleFunc("GET "+apiPath+"/config/storage", h.ReadAllStorage)
	mux.HandleFunc("POST "+apiPath+"/config/storage/{name}", h.AddStorage)
	mux.HandleFunc("GET "+apiPath+"/config/storage/{name}", h.ReadStorage)
	mux.HandleFunc("PUT "+apiPath+"/config/storage/{name}", h.UpdateStorage)
	mux.HandleFunc("DELETE "+apiPath+"/config/storage/{name}", h.DeleteStorage)
}

func registerPolicyRoutes(mux *http.ServeMux, apiPath string, h *handlers.Service) {
	mux.HandleFunc("GET "+apiPath+"/config/policies", h.ReadPolicies)
	mux.HandleFunc("POST "+apiPath+"/config/policies/{name}", h.AddPolicy)
	mux.HandleFunc("GET "+apiPath+"/config/policies/{name}", h.ReadPolicy)
	mux.HandleFunc("PUT "+apiPath+"/config/policies/{name}", h.UpdatePolicy)
	mux.HandleFunc("DELETE "+apiPath+"/config/policies/{name}", h.DeletePolicy)
}

func registerRoutineRoutes(mux *http.ServeMux, apiPath string, h *handlers.Service) {
	mux.HandleFunc("GET "+apiPath+"/config/routines", h.ReadRoutines)
	mux.HandleFunc("POST "+apiPath+"/config/routines/{name}", h.AddRoutine)
	mux.HandleFunc("GET "+apiPath+"/config/routines/{name}", h.ReadRoutine)
	mux.HandleFunc("PUT "+apiPath+"/config/routines/{name}", h.UpdateRoutine)
	mux.HandleFunc("DELETE "+apiPath+"/config/routines/{name}", h.DeleteRoutine)
	mux.HandleFunc("PUT "+apiPath+"/config/routines/{name}/disable", h.DisableRoutine)
	mux.HandleFunc("PUT "+apiPath+"/config/routines/{name}/enable", h.EnableRoutine)
}

func registerBackupRoutes(mux *http.ServeMux, apiPath string, h *handlers.Service) {
	mux.HandleFunc("GET "+apiPath+"/backups/full", h.GetAllFullBackups)
	mux.HandleFunc("GET "+apiPath+"/backups/full/{name}", h.GetFullBackupsForRoutine)
	mux.HandleFunc("GET "+apiPath+"/backups/incremental", h.GetAllIncrementalBackups)
	mux.HandleFunc("GET "+apiPath+"/backups/incremental/{name}", h.GetIncrementalBackupsForRoutine)
	mux.HandleFunc("POST "+apiPath+"/backups/schedule/{name}", h.ScheduleFullBackup)
	mux.HandleFunc("GET "+apiPath+"/backups/currentBackup/{name}", h.GetCurrentBackupInfo)
	mux.HandleFunc("POST "+apiPath+"/backups/cancel/{name}", h.CancelCurrentBackup)
}

func registerRestoreRoutes(mux *http.ServeMux, apiPath string, h *handlers.Service) {
	mux.HandleFunc("POST "+apiPath+"/restore/full", h.RestoreFullHandler)
	mux.HandleFunc("POST "+apiPath+"/restore/incremental", h.RestoreIncrementalHandler)
	mux.HandleFunc("POST "+apiPath+"/restore/timestamp", h.RestoreByTimeHandler)
	mux.HandleFunc("GET "+apiPath+"/restore/status/{jobId}", h.RestoreStatusHandler)
	mux.HandleFunc("POST "+apiPath+"/restore/cancel/{jobId}", h.CancelRestoreHandler)

	// Return backed up Aerospike configuration
	mux.HandleFunc("GET "+apiPath+"/retrieve/configuration/{name}/{timestamp}", h.RetrieveConfig)
}
