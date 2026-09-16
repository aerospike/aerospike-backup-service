package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPopInvalidatedRoutineNames_DrainsQueue(t *testing.T) {
	cfg := NewConfig()

	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1"}))
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r2"}))

	first := cfg.PopInvalidatedRoutineNames()
	assert.Equal(t, []string{"r1", "r2"}, first)

	second := cfg.PopInvalidatedRoutineNames()
	assert.Empty(t, second)
}

func TestPopInvalidatedRoutineNames_DeduplicatesNames(t *testing.T) {
	cfg := NewConfig()

	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1"}))
	cfg.invalidateRoutine("r1")
	cfg.invalidateRoutine("r1")

	invalidated := cfg.PopInvalidatedRoutineNames()
	assert.Equal(t, []string{"r1"}, invalidated)
}

func TestToggleRoutineDisabled_InvalidatesOnDisableAndEnable(t *testing.T) {
	cfg := NewConfig()
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1"}))
	cfg.PopInvalidatedRoutineNames() // clear AddRoutine invalidation

	require.NoError(t, cfg.ToggleRoutineDisabled("r1", true))
	require.NoError(t, cfg.ToggleRoutineDisabled("r1", false))

	invalidated := cfg.PopInvalidatedRoutineNames()
	assert.Equal(t, []string{"r1"}, invalidated)
}

func TestInvalidateRoutines(t *testing.T) {
	cfg := NewConfig()
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1"}))
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r2"}))
	cfg.PopInvalidatedRoutineNames()

	cfg.InvalidateRoutines([]string{"r1"})

	invalidated := cfg.PopInvalidatedRoutineNames()
	assert.Equal(t, []string{"r1"}, invalidated)
}

func TestInvalidateAllRoutines(t *testing.T) {
	cfg := NewConfig()
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1"}))
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r2"}))
	cfg.PopInvalidatedRoutineNames()

	cfg.InvalidateAllRoutines()

	invalidated := cfg.PopInvalidatedRoutineNames()
	assert.Equal(t, []string{"r1", "r2"}, invalidated)
}

func TestSetBackupConfig_DoesNotInvalidate(t *testing.T) {
	cfg := NewConfig()
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1"}))
	cfg.PopInvalidatedRoutineNames()

	other := cfg.BackupConfigCopy()
	cfg.SetBackupConfig(other)

	assert.Empty(t, cfg.PopInvalidatedRoutineNames())
}

func TestConfigAddRejectsNilValues(t *testing.T) {
	var typedNilStorage *LocalStorage

	tests := []struct {
		name    string
		add     func(*Config) error
		wantErr bool
	}{
		{
			name: "valid values",
			add: func(cfg *Config) error {
				if err := cfg.AddCluster("cluster", &AerospikeCluster{}); err != nil {
					return err
				}
				if err := cfg.AddRoutine(&BackupRoutine{Name: "routine"}); err != nil {
					return err
				}
				if err := cfg.AddPolicy("policy", &BackupPolicy{}); err != nil {
					return err
				}

				return cfg.AddStorage("storage", &LocalStorage{})
			},
		},
		{
			name:    "cluster",
			add:     func(cfg *Config) error { return cfg.AddCluster("cluster", nil) },
			wantErr: true,
		},
		{
			name:    "routine",
			add:     func(cfg *Config) error { return cfg.AddRoutine(nil) },
			wantErr: true,
		},
		{
			name:    "policy",
			add:     func(cfg *Config) error { return cfg.AddPolicy("policy", nil) },
			wantErr: true,
		},
		{
			name:    "storage",
			add:     func(cfg *Config) error { return cfg.AddStorage("storage", nil) },
			wantErr: true,
		},
		{
			name:    "typed nil storage",
			add:     func(cfg *Config) error { return cfg.AddStorage("typed-nil-storage", typedNilStorage) },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			err := tt.add(cfg)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
