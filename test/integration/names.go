//go:build integration

package integration

// Names baseConfig registers. Tests use them to reach into the configuration from a
// setupEnv customize function, and to address the data they seed.
const (
	clusterName     = "testCluster"
	storageName     = "local"
	policyName      = "defaultPolicy"
	routineName     = "integrationRoutine"
	secretAgentName = "secretAgent"
	namespace       = "test"
	setName         = "filteredSet"
)
