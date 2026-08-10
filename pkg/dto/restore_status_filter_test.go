package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStatusFilterFromString(t *testing.T) {
	t.Parallel()

	filter, err := NewStatusFilterFromString("running,success")
	require.NoError(t, err)
	assert.True(t, filter.Matches(model.RestoreRunning))
	assert.True(t, filter.Matches(model.RestoreSuccess))
	assert.False(t, filter.Matches(model.RestoreFailure))
}

func TestNewStatusFilterFromString_Exclude(t *testing.T) {
	t.Parallel()

	filter, err := NewStatusFilterFromString("!failure")
	require.NoError(t, err)
	assert.True(t, filter.Matches(model.RestoreRunning))
	assert.False(t, filter.Matches(model.RestoreFailure))
}

func TestNewStatusFilterFromString_EmptyAndWhitespace(t *testing.T) {
	t.Parallel()

	filter, err := NewStatusFilterFromString(" , ,")
	require.NoError(t, err)
	assert.True(t, filter.Matches(model.RestoreRunning))
}

func TestNewStatusFilterFromString_Invalid(t *testing.T) {
	t.Parallel()

	_, err := NewStatusFilterFromString("bogus-status")
	require.Error(t, err)
}
