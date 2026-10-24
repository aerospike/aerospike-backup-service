package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validConfig wires routine1 to cluster1, policy1 and storage1, so deleting any of those three
// hits the in-use check and updating any of them invalidates routine1.

func TestConfigDelete(t *testing.T) {
	t.Run("a routine is removed and left to be rescheduled", func(t *testing.T) {
		config := validConfig()

		invalidated, err := config.DeleteRoutine("routine1")

		require.NoError(t, err)
		assert.Equal(t, []string{"routine1"}, invalidated)
		assert.NotContains(t, config.BackupRoutines, "routine1")
	})

	t.Run("an entity no routine references is removed", func(t *testing.T) {
		config := validConfig()
		config.Storage["spare"] = &Storage{LocalStorage: &LocalStorage{Path: "/"}}

		invalidated, err := config.DeleteStorage("spare")

		require.NoError(t, err)
		assert.Empty(t, invalidated)
		assert.NotContains(t, config.Storage, "spare")
	})
}

func TestConfigDeleteUnknownName(t *testing.T) {
	tests := []struct {
		name   string
		delete func(*Config) ([]string, error)
	}{
		{"routine", func(c *Config) ([]string, error) { return c.DeleteRoutine("unknown") }},
		{"storage", func(c *Config) ([]string, error) { return c.DeleteStorage("unknown") }},
		{"cluster", func(c *Config) ([]string, error) { return c.DeleteCluster("unknown") }},
		{"policy", func(c *Config) ([]string, error) { return c.DeletePolicy("unknown") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.delete(validConfig())

			require.ErrorIs(t, err, model.ErrNotFound)
		})
	}
}

func TestConfigDeleteEntityInUse(t *testing.T) {
	tests := []struct {
		name    string
		delete  func(*Config) ([]string, error)
		survive func(*Config) bool
	}{
		{
			name:    "storage",
			delete:  func(c *Config) ([]string, error) { return c.DeleteStorage("storage1") },
			survive: func(c *Config) bool { return c.Storage["storage1"] != nil },
		},
		{
			name:    "cluster",
			delete:  func(c *Config) ([]string, error) { return c.DeleteCluster("cluster1") },
			survive: func(c *Config) bool { return c.AerospikeClusters["cluster1"] != nil },
		},
		{
			name:    "policy",
			delete:  func(c *Config) ([]string, error) { return c.DeletePolicy("policy1") },
			survive: func(c *Config) bool { return c.BackupPolicies["policy1"] != nil },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validConfig()

			_, err := tt.delete(config)

			require.ErrorIs(t, err, model.ErrInUse)
			assert.Contains(t, err.Error(), `routine "routine1"`)
			assert.True(t, tt.survive(config))
		})
	}
}

func TestConfigUpdateInvalidatesReferencingRoutines(t *testing.T) {
	tests := []struct {
		name   string
		update func(*Config) ([]string, error)
	}{
		{"routine", func(c *Config) ([]string, error) { return c.UpdateRoutine("routine1", &BackupRoutine{}) }},
		{"storage", func(c *Config) ([]string, error) { return c.UpdateStorage("storage1", &Storage{}) }},
		{"cluster", func(c *Config) ([]string, error) { return c.UpdateCluster("cluster1", &AerospikeCluster{}) }},
		{"policy", func(c *Config) ([]string, error) { return c.UpdatePolicy("policy1", &BackupPolicy{}) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invalidated, err := tt.update(validConfig())

			require.NoError(t, err)
			assert.Equal(t, []string{"routine1"}, invalidated)
		})
	}
}

func TestConfigUpdateUnknownName(t *testing.T) {
	tests := []struct {
		name   string
		update func(*Config) ([]string, error)
	}{
		{"routine", func(c *Config) ([]string, error) { return c.UpdateRoutine("unknown", &BackupRoutine{}) }},
		{"storage", func(c *Config) ([]string, error) { return c.UpdateStorage("unknown", &Storage{}) }},
		{"cluster", func(c *Config) ([]string, error) { return c.UpdateCluster("unknown", &AerospikeCluster{}) }},
		{"policy", func(c *Config) ([]string, error) { return c.UpdatePolicy("unknown", &BackupPolicy{}) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validConfig()

			_, err := tt.update(config)

			require.ErrorIs(t, err, model.ErrNotFound)
			assert.NotContains(t, config.BackupRoutines, "unknown")
		})
	}
}

func TestConfigAddExistingName(t *testing.T) {
	tests := []struct {
		name string
		add  func(*Config) ([]string, error)
	}{
		{"routine", func(c *Config) ([]string, error) { return c.AddRoutine("routine1", &BackupRoutine{}) }},
		{"storage", func(c *Config) ([]string, error) { return c.AddStorage("storage1", &Storage{}) }},
		{"cluster", func(c *Config) ([]string, error) { return c.AddCluster("cluster1", &AerospikeCluster{}) }},
		{"policy", func(c *Config) ([]string, error) { return c.AddPolicy("policy1", &BackupPolicy{}) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validConfig()

			_, err := tt.add(config)

			require.ErrorIs(t, err, model.ErrAlreadyExists)
			assert.Equal(t, "cluster1", config.BackupRoutines["routine1"].SourceCluster)
		})
	}
}

func TestConfigSetRoutineDisabled(t *testing.T) {
	config := validConfig()

	invalidated, err := config.SetRoutineDisabled("routine1", true)

	require.NoError(t, err)
	assert.Equal(t, []string{"routine1"}, invalidated)
	assert.True(t, config.BackupRoutines["routine1"].Disabled)

	_, err = config.SetRoutineDisabled("unknown", true)
	require.ErrorIs(t, err, model.ErrNotFound)
}
