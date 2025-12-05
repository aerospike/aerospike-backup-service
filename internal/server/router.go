package server

import (
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
)

func NewServeMux(apiPath, sysPath string, service *handlers.Service) *http.ServeMux {
	mux := http.NewServeMux()

	registerSystemRoutes(mux, sysPath)
	registerConfigRoutes(mux, apiPath, service)
	registerClusterRoutes(mux, apiPath, service)
	registerStorageRoutes(mux, apiPath, service)
	registerPolicyRoutes(mux, apiPath, service)
	registerRoutineRoutes(mux, apiPath, service)
	registerBackupRoutes(mux, apiPath, service)
	registerRestoreRoutes(mux, apiPath, service)

	return mux
}

func registerSystemRoutes(mux *http.ServeMux, sysPath string) {
	mux.HandleFunc("GET "+sysPath, handlers.RootActionHandler)
	mux.HandleFunc("GET "+sysPath+"health", handlers.HealthActionHandler)
	mux.HandleFunc("GET "+sysPath+"ready", handlers.ReadyActionHandler)
	mux.HandleFunc("GET "+sysPath+"version", handlers.VersionActionHandler)
	mux.Handle("GET "+sysPath+"metrics", handlers.MetricsActionHandler())
	mux.Handle("GET "+sysPath+"api-docs/", handlers.APIDocsActionHandler()) // Note the trailing slash
}

func registerConfigRoutes(mux *http.ServeMux, apiPath string, service *handlers.Service) {
	mux.HandleFunc("GET "+apiPath+"/config", service.ReadConfig)
	mux.HandleFunc("PUT "+apiPath+"/config", service.UpdateConfig)
	mux.HandleFunc("POST "+apiPath+"/config/apply", service.ApplyConfig)
}

func registerClusterRoutes(mux *http.ServeMux, apiPath string, service *handlers.Service) {
	mux.HandleFunc("GET "+apiPath+"/config/clusters", service.ReadAerospikeClusters)
	mux.HandleFunc("POST "+apiPath+"/config/clusters/{name}", service.AddAerospikeCluster)
	mux.HandleFunc("GET "+apiPath+"/config/clusters/{name}", service.ReadAerospikeCluster)
	mux.HandleFunc("PUT "+apiPath+"/config/clusters/{name}", service.UpdateAerospikeCluster)
	mux.HandleFunc("DELETE "+apiPath+"/config/clusters/{name}", service.DeleteAerospikeCluster)
	mux.HandleFunc("POST "+apiPath+"/config/clusters/check-connectivity", service.CheckAerospikeClusterConnectivity)
}

func registerStorageRoutes(mux *http.ServeMux, apiPath string, service *handlers.Service) {
	mux.HandleFunc("GET "+apiPath+"/config/storage", service.ReadAllStorage)
	mux.HandleFunc("POST "+apiPath+"/config/storage/{name}", service.AddStorage)
	mux.HandleFunc("GET "+apiPath+"/config/storage/{name}", service.ReadStorage)
	mux.HandleFunc("PUT "+apiPath+"/config/storage/{name}", service.UpdateStorage)
	mux.HandleFunc("DELETE "+apiPath+"/config/storage/{name}", service.DeleteStorage)
	mux.HandleFunc("POST "+apiPath+"/config/storage/check-connectivity", service.CheckStorageConnectivity)
}

func registerPolicyRoutes(mux *http.ServeMux, apiPath string, service *handlers.Service) {
	mux.HandleFunc("GET "+apiPath+"/config/policies", service.ReadPolicies)
	mux.HandleFunc("POST "+apiPath+"/config/policies/{name}", service.AddPolicy)
	mux.HandleFunc("GET "+apiPath+"/config/policies/{name}", service.ReadPolicy)
	mux.HandleFunc("PUT "+apiPath+"/config/policies/{name}", service.UpdatePolicy)
	mux.HandleFunc("DELETE "+apiPath+"/config/policies/{name}", service.DeletePolicy)
}

func registerRoutineRoutes(mux *http.ServeMux, apiPath string, service *handlers.Service) {
	mux.HandleFunc("GET "+apiPath+"/config/routines", service.ReadRoutines)
	mux.HandleFunc("POST "+apiPath+"/config/routines/{name}", service.AddRoutine)
	mux.HandleFunc("GET "+apiPath+"/config/routines/{name}", service.ReadRoutine)
	mux.HandleFunc("PUT "+apiPath+"/config/routines/{name}", service.UpdateRoutine)
	mux.HandleFunc("DELETE "+apiPath+"/config/routines/{name}", service.DeleteRoutine)
	mux.HandleFunc("PUT "+apiPath+"/config/routines/{name}/disable", service.DisableRoutine)
	mux.HandleFunc("PUT "+apiPath+"/config/routines/{name}/enable", service.EnableRoutine)
}

func registerBackupRoutes(mux *http.ServeMux, apiPath string, service *handlers.Service) {
	mux.HandleFunc("GET "+apiPath+"/backups/full", service.GetAllFullBackups)
	mux.HandleFunc("GET "+apiPath+"/backups/full/{name}", service.GetFullBackupsForRoutine)
	mux.HandleFunc("GET "+apiPath+"/backups/incremental", service.GetAllIncrementalBackups)
	mux.HandleFunc("GET "+apiPath+"/backups/incremental/{name}", service.GetIncrementalBackupsForRoutine)
	mux.HandleFunc("POST "+apiPath+"/backups/schedule/{name}", service.ScheduleFullBackup)
	mux.HandleFunc("GET "+apiPath+"/backups/currentBackup/{name}", service.GetCurrentBackupInfo)
	mux.HandleFunc("POST "+apiPath+"/backups/cancel/{name}", service.CancelCurrentBackup)
}

func registerRestoreRoutes(mux *http.ServeMux, apiPath string, service *handlers.Service) {
	mux.HandleFunc("POST "+apiPath+"/restore/full", service.RestoreFullHandler)
	mux.HandleFunc("POST "+apiPath+"/restore/incremental", service.RestoreIncrementalHandler)
	mux.HandleFunc("POST "+apiPath+"/restore/timestamp", service.RestoreByTimeHandler)
	mux.HandleFunc("GET "+apiPath+"/restore/status/{jobId}", service.RestoreStatusHandler)
	mux.HandleFunc("GET "+apiPath+"/restore/jobs", service.RetrieveRestoreJobs)
	mux.HandleFunc("POST "+apiPath+"/restore/cancel/{jobId}", service.CancelRestoreHandler)

	// Return backed up Aerospike configuration
	mux.HandleFunc("GET "+apiPath+"/retrieve/configuration/{name}/{timestamp}", service.RetrieveConfig)
}
