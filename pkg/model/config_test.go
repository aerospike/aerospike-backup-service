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

func TestSetBackupConfig_InvalidatesOnlyWhatChanged(t *testing.T) {
	cfg := NewConfig()
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1", IntervalCron: "@daily"}))
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r2", IntervalCron: "@daily"}))
	cfg.PopInvalidatedRoutineNames()

	next := cfg.BackupConfigCopy()
	next.BackupRoutines["r1"] = &BackupRoutine{Name: "r1", IntervalCron: "@hourly"}
	cfg.SetBackupConfig(next)

	assert.Equal(t, []string{"r1"}, cfg.PopInvalidatedRoutineNames())
}

func TestSetBackupConfig_IdenticalConfigurationInvalidatesNothing(t *testing.T) {
	cfg := NewConfig()
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1"}))
	cfg.PopInvalidatedRoutineNames()

	cfg.SetBackupConfig(cfg.BackupConfigCopy())

	assert.Empty(t, cfg.PopInvalidatedRoutineNames())
}

func TestSetBackupConfig_InvalidatesARemovedRoutine(t *testing.T) {
	cfg := NewConfig()
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1"}))
	cfg.PopInvalidatedRoutineNames()

	next := cfg.BackupConfigCopy()
	delete(next.BackupRoutines, "r1")
	cfg.SetBackupConfig(next)

	assert.Equal(t, []string{"r1"}, cfg.PopInvalidatedRoutineNames())
}

// A change whose apply never ran leaves its routines pending. The next change must not
// lose them just because it did not touch them itself.
func TestSetBackupConfig_KeepsPendingInvalidations(t *testing.T) {
	cfg := NewConfig()
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1", IntervalCron: "@daily"}))
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r2", IntervalCron: "@daily"}))
	cfg.PopInvalidatedRoutineNames()

	first := cfg.BackupConfigCopy()
	first.BackupRoutines["r1"] = &BackupRoutine{Name: "r1", IntervalCron: "@hourly"}
	cfg.SetBackupConfig(first)

	second := cfg.BackupConfigCopy()
	second.BackupRoutines["r2"] = &BackupRoutine{Name: "r2", IntervalCron: "@weekly"}
	cfg.SetBackupConfig(second)

	assert.Equal(t, []string{"r1", "r2"}, cfg.PopInvalidatedRoutineNames())
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
