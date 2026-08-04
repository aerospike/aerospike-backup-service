//go:build integration

package integration

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/app"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	clusterName = "testCluster"
	storageName = "local"
	policyName  = "defaultPolicy"
	routineName = "integrationRoutine"
	namespace   = "test"
	setName     = "filteredSet"
)

type env struct {
	backupDir string
	server    *httptest.Server
}

// setupEnv starts a backup service instance against the Aerospike container shared by the whole
// package. Each customize function receives the base configuration before it is written to disk,
// so a test can change any field without the fixture needing to know about it.
func setupEnv(t *testing.T, customize ...func(*dto.Config)) *env {
	t.Helper()

	ctx := t.Context()

	// Every test writes into the same namespace, so they must not call t.Parallel().
	require.NoError(t, asClient.Truncate(nil, namespace, "", nil))

	backupDir := t.TempDir()

	config := baseConfig(backupDir)
	for _, fn := range customize {
		fn(config)
	}

	configYAML, err := yaml.Marshal(config)
	require.NoError(t, err)

	configPath := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(configPath, configYAML, 0o600))

	scheduler, svc, err := app.InitComponents(ctx, configPath, false)
	require.NoError(t, err)

	scheduler.Start(ctx)
	t.Cleanup(func() { scheduler.Stop() })

	srv := httptest.NewServer(server.NewServeMux("/v1", "/", svc))
	t.Cleanup(srv.Close)

	return &env{
		backupDir: backupDir,
		server:    srv,
	}
}

// baseConfig is a minimal working configuration: one cluster, one local storage, one policy and
// one routine that only runs when triggered explicitly.
func baseConfig(backupDir string) *dto.Config {
	return &dto.Config{
		ServiceConfig: dto.ServiceConfig{
			Logger: &dto.LoggerConfig{Level: ptr.Of("ERROR")},
		},
		AerospikeClusters: map[string]*dto.AerospikeCluster{
			clusterName: {
				SeedNodes:            []dto.SeedNode{asSeedNode},
				UseServicesAlternate: ptr.Of(true),
			},
		},
		Storage: map[string]*dto.Storage{
			storageName: {
				LocalStorage: &dto.LocalStorage{Path: backupDir},
			},
		},
		BackupPolicies: map[string]*dto.BackupPolicy{
			policyName: {
				Parallel: ptr.Of(1),
				RetentionPolicy: &dto.RetentionPolicy{
					FullBackups: ptr.Of(10),
					IncrBackups: ptr.Of(0),
				},
			},
		},
		BackupRoutines: map[string]*dto.BackupRoutine{
			routineName: {
				BackupPolicy:  policyName,
				SourceCluster: clusterName,
				Storage:       storageName,
				IntervalCron:  "@yearly",
				Namespaces:    ptr.Of([]string{namespace}),
			},
		},
	}
}

// testRoutine returns the routine from baseConfig, for use inside setupEnv customize functions.
func testRoutine(c *dto.Config) *dto.BackupRoutine {
	return c.BackupRoutines[routineName]
}
