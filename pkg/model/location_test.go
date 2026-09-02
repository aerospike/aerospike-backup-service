package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTimezone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantNil bool
		wantErr string
	}{
		{name: "empty", input: "", wantNil: true},
		{name: "whitespace", input: " \t ", wantNil: true},
		{name: "utc keyword", input: "utc", want: "UTC"},
		{name: "UTC keyword", input: "UTC", want: "UTC"},
		{name: "local keyword", input: "local", want: "Local"},
		{name: "Local keyword", input: "Local", want: "Local"},
		{name: "iana", input: "America/New_York", want: "America/New_York"},
		{name: "EST rejected", input: "EST", wantErr: "EST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseTimezone(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, tt.want, got.String())
		})
	}
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

	t.Run("blank config defaults to UTC", func(t *testing.T) {
		t.Parallel()

		location := NewServiceLocation("")
		assert.Nil(t, location.resolved)
		assert.Equal(t, LocationSourceDefault, location.Source)
		assert.Same(t, DefaultScheduleTimezone, location.ResolvedLocation())
	})

	t.Run("explicit config uses service source", func(t *testing.T) {
		t.Parallel()

		location := NewServiceLocation("America/New_York")
		assert.Equal(t, "America/New_York", location.Configured)
		assert.Equal(t, LocationSourceService, location.Source)
		assert.Equal(t, "America/New_York", location.ResolvedLocation().String())
	})

	t.Run("local keyword is case-insensitive", func(t *testing.T) {
		t.Parallel()

		location := NewServiceLocation("local")
		assert.Equal(t, LocationSourceService, location.Source)
		assert.Equal(t, "Local", location.ResolvedLocation().String())
	})
}

func TestNewRoutineLocation(t *testing.T) {
	t.Parallel()

	service := NewServiceLocation("America/New_York")

	t.Run("inherits service timezone and source", func(t *testing.T) {
		t.Parallel()

		location := NewRoutineLocation("", service)
		assert.Equal(t, LocationSourceService, location.Source)
		assert.Equal(t, "America/New_York", location.ResolvedLocation().String())
	})

	t.Run("inherits UTC when service defaults to UTC", func(t *testing.T) {
		t.Parallel()

		location := NewRoutineLocation("", NewServiceLocation(""))
		assert.Equal(t, LocationSourceDefault, location.Source)
		assert.Nil(t, location.resolved)
		assert.Same(t, DefaultScheduleTimezone, location.ResolvedLocation())
	})

	t.Run("explicit routine override", func(t *testing.T) {
		t.Parallel()

		location := NewRoutineLocation("UTC", service)
		assert.Equal(t, "UTC", location.Configured)
		assert.Equal(t, LocationSourceRoutine, location.Source)
		assert.Same(t, DefaultScheduleTimezone, location.ResolvedLocation())
	})
}
