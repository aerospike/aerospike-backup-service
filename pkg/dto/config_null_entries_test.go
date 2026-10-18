package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A null routine entry ("backup-routines": {"x": null}) is a validation error, not a panic.
func TestConfigValidate_RejectsNilRoutine(t *testing.T) {
	c := &Config{BackupRoutines: map[string]*BackupRoutine{"x": nil}}

	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup routine 'x' validation error")
}

// A null secret-agent entry is rejected instead of being resolved, by pointer equality
// with nil, as the agent of every entity that has none.
func TestConfigValidate_RejectsNilSecretAgent(t *testing.T) {
	c := &Config{SecretAgents: map[string]*SecretAgent{"ghost": nil}}

	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret agent 'ghost' validation error")
}
