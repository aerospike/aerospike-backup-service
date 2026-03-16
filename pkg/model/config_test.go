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
	require.NoError(t, cfg.UpdateRoutine("r1", &BackupRoutine{Name: "r1"}))
	require.NoError(t, cfg.UpdateRoutine("r1", &BackupRoutine{Name: "r1"}))

	invalidated := cfg.PopInvalidatedRoutineNames()
	assert.Equal(t, []string{"r1"}, invalidated)
}

func TestUpdatePolicy_InvalidatesAffectedRoutines(t *testing.T) {
	cfg := NewConfig()
	oldPolicy := &BackupPolicy{}
	newPolicy := &BackupPolicy{}

	require.NoError(t, cfg.AddPolicy("p1", oldPolicy))
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1", BackupPolicy: oldPolicy}))
	cfg.PopInvalidatedRoutineNames() // clear AddRoutine invalidation

	require.NoError(t, cfg.UpdatePolicy("p1", newPolicy))

	invalidated := cfg.PopInvalidatedRoutineNames()
	assert.Equal(t, []string{"r1"}, invalidated)
}

func TestUpdateCluster_InvalidatesAffectedRoutines(t *testing.T) {
	cfg := NewConfig()
	oldCluster := &AerospikeCluster{}
	newCluster := &AerospikeCluster{}

	require.NoError(t, cfg.AddCluster("c1", oldCluster))
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1", SourceCluster: oldCluster}))
	cfg.PopInvalidatedRoutineNames() // clear AddRoutine invalidation

	require.NoError(t, cfg.UpdateCluster("c1", newCluster))

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

func TestDeleteRoutine_RecordsInvalidatedRoutineName(t *testing.T) {
	cfg := NewConfig()
	require.NoError(t, cfg.AddRoutine(&BackupRoutine{Name: "r1"}))
	cfg.PopInvalidatedRoutineNames() // clear AddRoutine invalidation

	require.NoError(t, cfg.DeleteRoutine("r1"))

	invalidated := cfg.PopInvalidatedRoutineNames()
	assert.Equal(t, []string{"r1"}, invalidated)
}
