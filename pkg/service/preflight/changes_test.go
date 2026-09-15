package preflight

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChangedOnly is the rule that replaced the per-handler "is this change worth
// validating" flag: probe what the change added or altered, and nothing else.
func TestChangedOnly(t *testing.T) {
	tests := []struct {
		name           string
		mutate         func(*testConfig)
		expectClusters []string
		expectStorage  []string
		expectRoutines []string
	}{
		{
			name:   "nothing changed",
			mutate: func(*testConfig) {},
		},
		{
			name:           "new cluster",
			mutate:         func(c *testConfig) { c.clusters["cluster3"] = newCluster(3200) },
			expectClusters: []string{"cluster3"},
		},
		{
			// The routine reading it comes along, because its namespaces are diffed
			// against whatever the edited cluster now reports.
			name:           "edited cluster",
			mutate:         func(c *testConfig) { c.clusters["cluster1"] = newCluster(3100) },
			expectClusters: []string{"cluster1"},
			expectRoutines: []string{"routine1"},
		},
		{
			name:          "new storage",
			mutate:        func(c *testConfig) { c.storages["storage3"] = &model.LocalStorage{Path: "/tmp/three"} },
			expectStorage: []string{"storage3"},
		},
		{
			name:          "edited storage",
			mutate:        func(c *testConfig) { c.storages["storage1"] = &model.LocalStorage{Path: "/tmp/moved"} },
			expectStorage: []string{"storage1"},
		},
		{
			// A credential rotation is invisible in the DTO, where secrets redact
			// themselves, and is exactly the change worth re-probing.
			name:          "rotated storage credential",
			mutate:        func(c *testConfig) { c.storages["bucket"] = newS3("rotated-key") },
			expectStorage: []string{"bucket"},
		},
		{
			name:           "routine namespaces edited",
			mutate:         func(c *testConfig) { c.routines["routine1"].namespaces = []string{"ns-other"} },
			expectClusters: []string{"cluster1"},
			expectRoutines: []string{"routine1"},
		},
		{
			name:           "routine repointed at another existing cluster",
			mutate:         func(c *testConfig) { c.routines["routine1"].cluster = "cluster2" },
			expectClusters: []string{"cluster2"},
			expectRoutines: []string{"routine1"},
		},
		{
			name:          "routine repointed at another existing storage",
			mutate:        func(c *testConfig) { c.routines["routine1"].storage = "storage2" },
			expectStorage: []string{"storage2"},
		},
		{
			// Everything a delete leaves behind was already there.
			name:   "cluster deleted",
			mutate: func(c *testConfig) { delete(c.clusters, "cluster2") },
		},
		{
			name:   "storage deleted",
			mutate: func(c *testConfig) { delete(c.storages, "storage2") },
		},
		{
			name:   "routine deleted",
			mutate: func(c *testConfig) { delete(c.routines, "routine1") },
		},
		{
			// The cases the opt-out flag used to exist for, now falling out of the rule.
			name:   "routine disabled",
			mutate: func(c *testConfig) { c.routines["routine1"].disabled = true },
		},
		{
			name:   "routine enabled",
			mutate: func(c *testConfig) { c.routines["routine1"].disabled = false },
		},
		{
			name:   "policy edited",
			mutate: func(c *testConfig) { c.routines["routine1"].parallel = 16 },
		},
		{
			name:   "routine schedule edited",
			mutate: func(c *testConfig) { c.routines["routine1"].cron = "@hourly" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := baseConfig()

			changed := baseConfig()
			tt.mutate(changed)

			delta := changedOnly(previous.build(t), changed.build(t)).BackupConfigCopy()

			assert.ElementsMatch(t, tt.expectClusters, names(delta.AerospikeClusters), "clusters")
			assert.ElementsMatch(t, tt.expectStorage, names(delta.Storage), "storage")
			assert.ElementsMatch(t, tt.expectRoutines, names(delta.BackupRoutines), "routines")
		})
	}
}

// A first check has nothing to compare against, so everything counts as new.
func TestChangedOnly_NoPrevious(t *testing.T) {
	delta := changedOnly(nil, baseConfig().build(t)).BackupConfigCopy()

	assert.ElementsMatch(t, []string{"cluster1", "cluster2"}, names(delta.AerospikeClusters))
	assert.ElementsMatch(t, []string{"storage1", "storage2", "bucket"}, names(delta.Storage))
}

func TestChangedOnly_NoCurrent(t *testing.T) {
	delta := changedOnly(baseConfig().build(t), nil).BackupConfigCopy()

	assert.Empty(t, delta.AerospikeClusters)
	assert.Empty(t, delta.Storage)
}

// testConfig is the entity graph a configuration is built from. Routines name what they
// reference, and build resolves those names to pointers, the way the DTO layer does: a
// routine follows an edited cluster without the test having to repoint it by hand.
//
// Each call to baseConfig allocates a fresh graph, so comparing two of them compares
// values and never pointers - the situation after a real change, where the DTO round trip
// rebuilds every entity.
type testConfig struct {
	clusters map[string]*model.AerospikeCluster
	storages map[string]model.Storage
	routines map[string]*testRoutine
}

type testRoutine struct {
	cluster    string
	storage    string
	namespaces []string
	cron       string
	parallel   int
	disabled   bool
}

func baseConfig() *testConfig {
	c := &testConfig{
		clusters: map[string]*model.AerospikeCluster{
			"cluster1": newCluster(3000),
			"cluster2": newCluster(3001),
		},
		storages: map[string]model.Storage{
			"storage1": &model.LocalStorage{Path: "/tmp/one"},
			"storage2": &model.LocalStorage{Path: "/tmp/two"},
			"bucket":   newS3("key-1"),
		},
	}

	c.routines = map[string]*testRoutine{
		"routine1": {
			cluster:    "cluster1",
			storage:    "storage1",
			namespaces: []string{"ns1"},
			cron:       "@daily",
			parallel:   8,
		},
	}

	return c
}

func (c *testConfig) build(t *testing.T) *model.Config {
	t.Helper()

	config := model.NewConfig()
	for name, cluster := range c.clusters {
		require.NoError(t, config.AddCluster(name, cluster))
	}
	for name, storage := range c.storages {
		require.NoError(t, config.AddStorage(name, storage))
	}
	for name, routine := range c.routines {
		require.NoError(t, config.AddRoutine(&model.BackupRoutine{
			Name:          name,
			BackupPolicy:  &model.BackupPolicy{Parallel: ptrTo(routine.parallel)},
			SourceCluster: c.clusters[routine.cluster],
			Storage:       c.storages[routine.storage],
			IntervalCron:  routine.cron,
			Namespaces:    routine.namespaces,
			Disabled:      routine.disabled,
		}))
	}

	return config
}

func newCluster(port model.Port) *model.AerospikeCluster {
	return &model.AerospikeCluster{
		SeedNodes: []model.SeedNode{{HostName: "localhost", Port: port}},
	}
}

func newS3(key model.Secret) model.Storage {
	return &model.S3Storage{
		Bucket: "bucket",
		Auth:   &model.S3Authentication{KeyIDSecret: key, AccessKeySecret: "secret"},
	}
}

func ptrTo[T any](v T) *T { return &v }

func names[T any](entries map[string]T) []string {
	if len(entries) == 0 {
		return nil
	}

	result := make([]string, 0, len(entries))
	for name := range entries {
		result = append(result, name)
	}

	return result
}
