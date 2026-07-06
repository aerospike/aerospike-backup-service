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

func TestSetBackupConfig_DoesNotInvalidate(t *testing.T) {
	cfg := NewConfig()
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1"}))
	cfg.PopInvalidatedRoutineNames()

	other := cfg.BackupConfigCopy()
	cfg.SetBackupConfig(other)

	assert.Empty(t, cfg.PopInvalidatedRoutineNames())
}
