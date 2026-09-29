package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTimeBoundsFromString(t *testing.T) {
	t.Parallel()

	bounds, err := NewTimeBoundsFromString("1000", "2000")
	require.NoError(t, err)
	require.NotNil(t, bounds.FromTime)
	require.NotNil(t, bounds.ToTime)
	assert.Equal(t, time.UnixMilli(1000), *bounds.FromTime)
	assert.Equal(t, time.UnixMilli(2000), *bounds.ToTime)
}

func TestNewTimeBoundsFromString_Empty(t *testing.T) {
	t.Parallel()

	bounds, err := NewTimeBoundsFromString("", "")
	require.NoError(t, err)
	assert.Nil(t, bounds.FromTime)
	assert.Nil(t, bounds.ToTime)
}

func TestNewTimeBoundsFromString_InvalidFrom(t *testing.T) {
	t.Parallel()

	_, err := NewTimeBoundsFromString("not-a-number", "2000")
	require.Error(t, err)
}

func TestNewTimeBoundsFromString_InvalidTo(t *testing.T) {
	t.Parallel()

	_, err := NewTimeBoundsFromString("1000", "not-a-number")
	require.Error(t, err)
}

func TestNewTimeBoundsFromString_Negative(t *testing.T) {
	t.Parallel()

	_, err := NewTimeBoundsFromString("-5", "2000")
	require.Error(t, err)
}
