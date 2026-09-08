package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustResolve panics on error. Test helper for known-good timezone names.
func mustResolve(configured string) *time.Location {
	loc, err := ResolveTimezone(configured)
	if err != nil {
		panic(err)
	}

	return loc
}

// mustServiceLocation builds a service-level Location from a known-good value.
func mustServiceLocation(configured string) Location {
	return NewServiceLocation(configured, mustResolve(configured))
}

// mustRoutineLocation builds a routine-level Location from a known-good value.
func mustRoutineLocation(configured string, service Location) Location {
	return NewRoutineLocation(configured, mustResolve(configured), service)
}

func TestLocation_ResolvedLocation(t *testing.T) {
	t.Parallel()

	assert.Same(t, DefaultScheduleTimezone, Location{}.ResolvedLocation())

	ny, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	location := Location{resolved: ny}
	assert.Same(t, ny, location.ResolvedLocation())
}

func TestNewServiceLocation(t *testing.T) {
	t.Parallel()

	t.Run("nil resolved defaults to UTC", func(t *testing.T) {
		t.Parallel()

		location := NewServiceLocation("", nil)
		assert.False(t, location.IsExplicit())
		assert.Empty(t, location.Configured)
		assert.Same(t, DefaultScheduleTimezone, location.ResolvedLocation())
	})

	t.Run("resolved iana is explicit", func(t *testing.T) {
		t.Parallel()

		location := mustServiceLocation("America/New_York")
		assert.True(t, location.IsExplicit())
		assert.Equal(t, "America/New_York", location.Configured)
		assert.Equal(t, "America/New_York", location.ResolvedLocation().String())
	})

	t.Run("resolved local keeps configured spelling", func(t *testing.T) {
		t.Parallel()

		location := mustServiceLocation("local")
		assert.True(t, location.IsExplicit())
		assert.Equal(t, "local", location.Configured)
		assert.Equal(t, "Local", location.ResolvedLocation().String())
	})
}

func TestNewRoutineLocation(t *testing.T) {
	t.Parallel()

	service := mustServiceLocation("America/New_York")

	t.Run("nil resolved inherits service resolved timezone", func(t *testing.T) {
		t.Parallel()

		location := NewRoutineLocation("", nil, service)
		assert.False(t, location.IsExplicit())
		assert.Empty(t, location.Configured)
		assert.Equal(t, "America/New_York", location.ResolvedLocation().String())
	})

	t.Run("inherits UTC when service defaults to UTC", func(t *testing.T) {
		t.Parallel()

		location := NewRoutineLocation("", nil, NewServiceLocation("", nil))
		assert.False(t, location.IsExplicit())
		assert.Nil(t, location.resolved)
		assert.Same(t, DefaultScheduleTimezone, location.ResolvedLocation())
	})

	t.Run("explicit routine override", func(t *testing.T) {
		t.Parallel()

		location := mustRoutineLocation("UTC", service)
		assert.True(t, location.IsExplicit())
		assert.Equal(t, "UTC", location.Configured)
		assert.Same(t, DefaultScheduleTimezone, location.ResolvedLocation())
	})
}

func TestResolveTimezone(t *testing.T) {
	t.Parallel()

	empty, err := ResolveTimezone("")
	require.NoError(t, err)
	assert.Nil(t, empty)

	utc, err := ResolveTimezone("utc")
	require.NoError(t, err)
	assert.Same(t, time.UTC, utc)

	ny, err := ResolveTimezone("America/New_York")
	require.NoError(t, err)
	assert.Equal(t, "America/New_York", ny.String())

	_, err = ResolveTimezone("Not/AZone")
	require.Error(t, err)
}
