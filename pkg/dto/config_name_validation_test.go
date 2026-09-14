package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Entity names (map keys) and namespace names end up in storage paths through PathService, so
// validation must accept only a single path segment: no separators, no "..".
func TestConfigValidate_RejectsPathSegmentsInNames(t *testing.T) {
	t.Skip("BKRS-414: names and namespaces are checked only for emptiness")

	config := func(routineName string, namespaces []string) *Config {
		return &Config{
			AerospikeClusters: map[string]*AerospikeCluster{
				"c": {SeedNodes: []SeedNode{{HostName: "localhost", Port: 3000}}},
			},
			Storage: map[string]*Storage{
				"s": {LocalStorage: &LocalStorage{Path: "/var/backups"}},
			},
			BackupRoutines: map[string]*BackupRoutine{
				routineName: {
					SourceCluster: "c",
					Storage:       "s",
					IntervalCron:  "@daily",
					Namespaces:    &namespaces,
				},
			},
		}
	}

	require.Error(t, config("../../escaped-routine", []string{"ns"}).Validate(), "routine name with path traversal")
	require.Error(t, config("a/b", []string{"ns"}).Validate(), "routine name with a path separator")
	require.Error(t, config("r", []string{"../../escaped-ns"}).Validate(), "namespace with path traversal")
	require.NoError(t, config("daily-backup", []string{"ns"}).Validate())
}
