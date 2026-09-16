package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deletableConfig holds one routine and, for each kind of entry, one the routine refers to
// and one nothing does.
func deletableConfig() *Config {
	return &Config{
		AerospikeClusters: map[string]*AerospikeCluster{"used-cluster": {}, "spare-cluster": {}},
		Storage:           map[string]*Storage{"used-storage": {}, "spare-storage": {}},
		BackupPolicies:    map[string]*BackupPolicy{"used-policy": {}, "spare-policy": {}},
		BackupRoutines: map[string]*BackupRoutine{
			"nightly": {SourceCluster: "used-cluster", Storage: "used-storage", BackupPolicy: "used-policy"},
		},
	}
}

func TestConfig_Delete(t *testing.T) {
	tests := []struct {
		name    string
		del     func(*Config) error
		gone    func(*Config) bool
		wantErr error
	}{
		{
			name: "cluster nothing refers to",
			del:  func(c *Config) error { return c.DeleteCluster("spare-cluster") },
			gone: func(c *Config) bool { _, ok := c.AerospikeClusters["spare-cluster"]; return !ok },
		},
		{
			name: "storage nothing refers to",
			del:  func(c *Config) error { return c.DeleteStorage("spare-storage") },
			gone: func(c *Config) bool { _, ok := c.Storage["spare-storage"]; return !ok },
		},
		{
			name: "policy nothing refers to",
			del:  func(c *Config) error { return c.DeletePolicy("spare-policy") },
			gone: func(c *Config) bool { _, ok := c.BackupPolicies["spare-policy"]; return !ok },
		},
		{
			name: "routine",
			del:  func(c *Config) error { return c.DeleteRoutine("nightly") },
			gone: func(c *Config) bool { _, ok := c.BackupRoutines["nightly"]; return !ok },
		},
		{
			name:    "cluster a routine reads from",
			del:     func(c *Config) error { return c.DeleteCluster("used-cluster") },
			wantErr: model.ErrInUse,
		},
		{
			name:    "storage a routine writes to",
			del:     func(c *Config) error { return c.DeleteStorage("used-storage") },
			wantErr: model.ErrInUse,
		},
		{
			name:    "policy a routine uses",
			del:     func(c *Config) error { return c.DeletePolicy("used-policy") },
			wantErr: model.ErrInUse,
		},
		{
			name:    "unknown cluster",
			del:     func(c *Config) error { return c.DeleteCluster("missing") },
			wantErr: model.ErrNotFound,
		},
		{
			name:    "unknown storage",
			del:     func(c *Config) error { return c.DeleteStorage("missing") },
			wantErr: model.ErrNotFound,
		},
		{
			name:    "unknown policy",
			del:     func(c *Config) error { return c.DeletePolicy("missing") },
			wantErr: model.ErrNotFound,
		},
		{
			name:    "unknown routine",
			del:     func(c *Config) error { return c.DeleteRoutine("missing") },
			wantErr: model.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := deletableConfig()
			err := tt.del(config)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.True(t, tt.gone(config))
		})
	}
}

// The in-use error names the routine, which is what the operator has to change first.
func TestConfig_DeleteInUse_NamesTheRoutine(t *testing.T) {
	err := deletableConfig().DeleteCluster("used-cluster")

	require.ErrorContains(t, err, `it is used in routine "nightly"`)
}
