package server

import (
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
)

// Route is one endpoint the service serves.
type Route struct {
	// Method is the HTTP method, e.g. "GET".
	Method string
	// Pattern is the path as registered, including the context path prefix.
	Pattern string
	// Handler serves the route.
	Handler http.Handler
}

// NewServeMux registers every route returned by Routes.
func NewServeMux(apiPath, sysPath string, service *handlers.Service) *http.ServeMux {
	mux := http.NewServeMux()

	for _, route := range Routes(apiPath, sysPath, service) {
		mux.Handle(route.Method+" "+route.Pattern, route.Handler)
	}

	return mux
}

// Routes returns every endpoint the service serves, in registration order.
//
// The service may be nil when only the methods and patterns are of interest:
// building a method value from a nil receiver is legal, and nothing here calls
// the handlers.
func Routes(apiPath, sysPath string, service *handlers.Service) []Route {
	routes := systemRoutes(sysPath)
	routes = append(routes, configRoutes(apiPath, service)...)
	routes = append(routes, clusterRoutes(apiPath, service)...)
	routes = append(routes, storageRoutes(apiPath, service)...)
	routes = append(routes, policyRoutes(apiPath, service)...)
	routes = append(routes, routineRoutes(apiPath, service)...)
	routes = append(routes, backupRoutes(apiPath, service)...)
	routes = append(routes, restoreRoutes(apiPath, service)...)

	return routes
}

func systemRoutes(sysPath string) []Route {
	return []Route{
		get(sysPath, handlers.RootActionHandler),
		get(sysPath+"health", handlers.HealthActionHandler),
		get(sysPath+"ready", handlers.ReadyActionHandler),
		get(sysPath+"version", handlers.VersionActionHandler),
		{Method: http.MethodGet, Pattern: sysPath + "metrics", Handler: handlers.MetricsActionHandler()},
		// Note the trailing slash: the API docs are served as a subtree.
		{Method: http.MethodGet, Pattern: sysPath + "api-docs/", Handler: handlers.APIDocsActionHandler()},
	}
}

func configRoutes(apiPath string, service *handlers.Service) []Route {
	return []Route{
		get(apiPath+"/config", service.ReadConfig),
		put(apiPath+"/config", service.UpdateConfig),
		post(apiPath+"/config/apply", service.ApplyConfig),
	}
}

func clusterRoutes(apiPath string, service *handlers.Service) []Route {
	return []Route{
		get(apiPath+"/config/clusters", service.ReadAerospikeClusters),
		post(apiPath+"/config/clusters/{name}", service.AddAerospikeCluster),
		get(apiPath+"/config/clusters/{name}", service.ReadAerospikeCluster),
		put(apiPath+"/config/clusters/{name}", service.UpdateAerospikeCluster),
		del(apiPath+"/config/clusters/{name}", service.DeleteAerospikeCluster),
	}
}

func storageRoutes(apiPath string, service *handlers.Service) []Route {
	return []Route{
		get(apiPath+"/config/storage", service.ReadAllStorage),
		post(apiPath+"/config/storage/{name}", service.AddStorage),
		get(apiPath+"/config/storage/{name}", service.ReadStorage),
		put(apiPath+"/config/storage/{name}", service.UpdateStorage),
		del(apiPath+"/config/storage/{name}", service.DeleteStorage),
	}
}

func policyRoutes(apiPath string, service *handlers.Service) []Route {
	return []Route{
		get(apiPath+"/config/policies", service.ReadPolicies),
		post(apiPath+"/config/policies/{name}", service.AddPolicy),
		get(apiPath+"/config/policies/{name}", service.ReadPolicy),
		put(apiPath+"/config/policies/{name}", service.UpdatePolicy),
		del(apiPath+"/config/policies/{name}", service.DeletePolicy),
	}
}

func routineRoutes(apiPath string, service *handlers.Service) []Route {
	return []Route{
		get(apiPath+"/config/routines", service.ReadRoutines),
		post(apiPath+"/config/routines/{name}", service.AddRoutine),
		get(apiPath+"/config/routines/{name}", service.ReadRoutine),
		put(apiPath+"/config/routines/{name}", service.UpdateRoutine),
		del(apiPath+"/config/routines/{name}", service.DeleteRoutine),
		put(apiPath+"/config/routines/{name}/disable", service.DisableRoutine),
		put(apiPath+"/config/routines/{name}/enable", service.EnableRoutine),
	}
}

func backupRoutes(apiPath string, service *handlers.Service) []Route {
	return []Route{
		get(apiPath+"/backups/full", service.GetAllFullBackups),
		get(apiPath+"/backups/full/{name}", service.GetFullBackupsForRoutine),
		get(apiPath+"/backups/incremental", service.GetAllIncrementalBackups),
		get(apiPath+"/backups/incremental/{name}", service.GetIncrementalBackupsForRoutine),
		post(apiPath+"/backups/full/{name}", service.TriggerFullBackup),
		post(apiPath+"/backups/incremental/{name}", service.TriggerIncrementalBackup),
		post(apiPath+"/backups/schedule/{name}", service.ScheduleFullBackup),
		get(apiPath+"/backups/currentBackup/{name}", service.GetCurrentBackupInfo),
		post(apiPath+"/backups/cancel/{name}", service.CancelCurrentBackup),
	}
}

func restoreRoutes(apiPath string, service *handlers.Service) []Route {
	return []Route{
		post(apiPath+"/restore/full", service.RestoreFullHandler),
		post(apiPath+"/restore/incremental", service.RestoreIncrementalHandler),
		post(apiPath+"/restore/timestamp", service.RestoreByTimeHandler),
		get(apiPath+"/restore/status/{jobId}", service.RestoreStatusHandler),
		get(apiPath+"/restore/jobs", service.RetrieveRestoreJobs),
		post(apiPath+"/restore/cancel/{jobId}", service.CancelRestoreHandler),

		// Return backed up Aerospike configuration.
		get(apiPath+"/retrieve/configuration/{name}/{timestamp}", service.RetrieveConfig),
	}
}

func get(pattern string, handler http.HandlerFunc) Route {
	return Route{Method: http.MethodGet, Pattern: pattern, Handler: handler}
}

func post(pattern string, handler http.HandlerFunc) Route {
	return Route{Method: http.MethodPost, Pattern: pattern, Handler: handler}
}

func put(pattern string, handler http.HandlerFunc) Route {
	return Route{Method: http.MethodPut, Pattern: pattern, Handler: handler}
}

func del(pattern string, handler http.HandlerFunc) Route {
	return Route{Method: http.MethodDelete, Pattern: pattern, Handler: handler}
}
