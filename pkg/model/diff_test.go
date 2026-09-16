package model

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testBackupConfig builds a configuration the way ToModel builds one: the routine points at
// the very cluster, policy and storage the configuration holds by name, and every call
// allocates a fresh set, so two calls are equal in value and share no pointer.
func testBackupConfig() *BackupConfig {
	cluster := &AerospikeCluster{SeedNodes: []SeedNode{{HostName: "localhost", Port: 3000}}}
	policy := &BackupPolicy{Parallel: ptr.Of(8)}
	storage := &LocalStorage{Path: "/data/backup"}

	config := newBackupConfig()
	config.AerospikeClusters["cluster1"] = cluster
	config.BackupPolicies["policy1"] = policy
	config.Storage["storage1"] = storage
	config.Storage["unused"] = &LocalStorage{Path: "/data/unused"}
	config.BackupRoutines["routine1"] = &BackupRoutine{
		Name:          "routine1",
		BackupPolicy:  policy,
		SourceCluster: cluster,
		Storage:       storage,
		IntervalCron:  "@daily",
	}

	return config
}

func TestChangedRoutines(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*BackupConfig)
		changed []string
	}{
		{
			name:    "a configuration nobody edited has no changed routines",
			change:  func(*BackupConfig) {},
			changed: nil,
		},
		{
			name:    "an edit to the routine itself",
			change:  func(c *BackupConfig) { c.BackupRoutines["routine1"].IntervalCron = "@hourly" },
			changed: []string{"routine1"},
		},
		{
			name:    "an edit to the cluster the routine reads",
			change:  func(c *BackupConfig) { c.AerospikeClusters["cluster1"].SeedNodes[0].Port = 4000 },
			changed: []string{"routine1"},
		},
		{
			name:    "an edit to the policy the routine uses",
			change:  func(c *BackupConfig) { c.BackupPolicies["policy1"].Parallel = ptr.Of(2) },
			changed: []string{"routine1"},
		},
		{
			name:    "an edit to the storage the routine writes to",
			change:  func(c *BackupConfig) { c.Storage["storage1"].(*LocalStorage).Path = "/data/other" },
			changed: []string{"routine1"},
		},
		{
			name: "a routine added",
			change: func(c *BackupConfig) {
				c.BackupRoutines["routine2"] = &BackupRoutine{Name: "routine2", IntervalCron: "@weekly"}
			},
			changed: []string{"routine2"},
		},
		{
			name:    "a routine removed",
			change:  func(c *BackupConfig) { delete(c.BackupRoutines, "routine1") },
			changed: []string{"routine1"},
		},
		{
			name:    "an edit to storage no routine writes to",
			change:  func(c *BackupConfig) { c.Storage["unused"].(*LocalStorage).Path = "/data/elsewhere" },
			changed: nil,
		},
		{
			name:    "a cluster added that no routine reads",
			change:  func(c *BackupConfig) { c.AerospikeClusters["spare"] = &AerospikeCluster{} },
			changed: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := testBackupConfig()
			current := testBackupConfig()
			tt.change(current)

			assert.Equal(t, tt.changed, ChangedRoutines(current, previous))
			// The comparison is symmetric, so a caller cannot get it wrong by argument order.
			assert.Equal(t, tt.changed, ChangedRoutines(previous, current))
		})
	}
}

// An edit to a shared cluster reaches every routine that reads it, and only those.
func TestChangedRoutines_ReachesEveryRoutineThatSharesTheEditedEntry(t *testing.T) {
	previous := testBackupConfig()
	addRoutineReadingCluster(previous, "routine2", previous.AerospikeClusters["cluster1"])
	addRoutineReadingCluster(previous, "isolated", &AerospikeCluster{
		SeedNodes: []SeedNode{{HostName: "elsewhere", Port: 3000}},
	})

	current := testBackupConfig()
	addRoutineReadingCluster(current, "routine2", current.AerospikeClusters["cluster1"])
	addRoutineReadingCluster(current, "isolated", &AerospikeCluster{
		SeedNodes: []SeedNode{{HostName: "elsewhere", Port: 3000}},
	})
	current.AerospikeClusters["cluster1"].SeedNodes[0].Port = 4000

	assert.Equal(t, []string{"routine1", "routine2"}, ChangedRoutines(current, previous))
}

func addRoutineReadingCluster(config *BackupConfig, name string, cluster *AerospikeCluster) {
	config.BackupRoutines[name] = &BackupRoutine{
		Name:          name,
		BackupPolicy:  config.BackupPolicies["policy1"],
		SourceCluster: cluster,
		Storage:       config.Storage["storage1"],
		IntervalCron:  "@daily",
	}
}

// Diff reports what the receiver holds, so a removed entry shows up only when the two
// configurations are passed the other way round. ChangedRoutines is what runs both.
func TestDiff_ReportsOnlyWhatTheReceiverHolds(t *testing.T) {
	previous := testBackupConfig()
	current := testBackupConfig()
	delete(current.BackupRoutines, "routine1")

	assert.Empty(t, current.Diff(previous).BackupRoutines)
	assert.Contains(t, previous.Diff(current).BackupRoutines, "routine1")
}

// Every kind of entry a configuration holds takes part in the comparison.
func TestDiff_ReportsEntriesOfEveryKind(t *testing.T) {
	previous := newBackupConfig()
	current := testBackupConfig()
	current.SecretAgents["agent1"] = &SecretAgent{Address: "localhost"}

	delta := current.Diff(previous)

	assert.Contains(t, delta.AerospikeClusters, "cluster1")
	assert.Contains(t, delta.BackupPolicies, "policy1")
	assert.Contains(t, delta.Storage, "storage1")
	assert.Contains(t, delta.BackupRoutines, "routine1")
	assert.Contains(t, delta.SecretAgents, "agent1")
}

// A routine carries a resolved timezone holding a *time.Location, which has to compare
// equal across two configurations that name the same zone.
func TestChangedRoutines_SameTimezoneIsNotAChange(t *testing.T) {
	previous := testBackupConfig()
	current := testBackupConfig()

	previous.BackupRoutines["routine1"].Timezone = routineLocation(t, "America/New_York")
	current.BackupRoutines["routine1"].Timezone = routineLocation(t, "America/New_York")

	assert.Empty(t, ChangedRoutines(current, previous))
}

func TestDiff_NilConfigurations(t *testing.T) {
	var absent *BackupConfig

	assert.Empty(t, absent.Diff(testBackupConfig()).BackupRoutines)
	assert.Contains(t, testBackupConfig().Diff(absent).BackupRoutines, "routine1")
	assert.Equal(t, []string{"routine1"}, ChangedRoutines(absent, testBackupConfig()))
	assert.Empty(t, ChangedRoutines(absent, nil))
}

// routineLocation resolves a timezone the way the DTO layer resolves one, allocating a
// separate *time.Location per call.
func routineLocation(t *testing.T, zone string) Location {
	t.Helper()

	resolved, err := ResolveTimezone(zone)
	require.NoError(t, err)

	return NewRoutineLocation(zone, resolved, Location{})
}
