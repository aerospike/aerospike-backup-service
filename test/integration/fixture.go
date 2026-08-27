//go:build integration

package integration

import (
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v3/internal/app"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/prometheus"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	as "github.com/aerospike/aerospike-client-go/v8"
)

// env is a running backup service instance under test.
type env struct {
	backupDir string
	server    *httptest.Server
}

// setupEnv starts a backup service against the suite's Aerospike container. Each customize
// function receives the base configuration before it is written to disk, so a test can change any
// field without this harness needing to know about it.
func (s *Suite) setupEnv(customize ...func(*dto.Config)) *env {
	t := s.T()
	ctx := t.Context()

	backupDir := t.TempDir()

	config := s.baseConfig(backupDir)
	for _, fn := range customize {
		fn(config)
	}

	configYAML, err := decoder.Marshal(config, decoder.YAML, false)
	s.Require().NoError(err)

	configPath := filepath.Join(t.TempDir(), "config.yml")
	s.Require().NoError(os.WriteFile(configPath, configYAML, 0o600))

	components, err := app.InitComponents(ctx, configPath, false)
	s.Require().NoError(err)

	components.Scheduler.Start(ctx)
	components.MetricsCollector.Start(ctx, prometheus.CollectInterval)
	t.Cleanup(func() { components.Scheduler.Stop() })

	srv := httptest.NewServer(components.Servers[0]) // only http sever is configured.
	t.Cleanup(srv.Close)

	return &env{
		backupDir: backupDir,
		server:    srv,
	}
}

// baseConfig is a minimal working configuration: one cluster, one local storage, one policy and
// one routine that only runs when triggered explicitly.
func (s *Suite) baseConfig(backupDir string) *dto.Config {
	return &dto.Config{
		ServiceConfig: dto.ServiceConfig{
			Logger: &dto.LoggerConfig{Level: "ERROR"},
		},
		AerospikeClusters: map[string]*dto.AerospikeCluster{
			clusterName: {
				SeedNodes:            []dto.SeedNode{s.seedNode},
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

// seedRecords writes one record per age into the set that tests back up.
func (s *Suite) seedRecords(ages []int) {
	writePolicy := as.NewWritePolicy(0, 0)

	for i, age := range ages {
		key, err := as.NewKey(namespace, setName, i)
		s.Require().NoError(err)
		s.Require().NoError(s.client.Put(writePolicy, key, as.BinMap{"age": age}))
	}
}
