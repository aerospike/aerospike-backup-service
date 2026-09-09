package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduleTimezone_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   ScheduleTimezone
		wantErr string
	}{
		{name: "empty", value: ""},
		{name: "whitespace", value: " \t "},
		{name: "utc keyword", value: "utc"},
		{name: "UTC keyword", value: "UTC"},
		{name: "local keyword", value: "local"},
		{name: "Local keyword", value: "Local"},
		{name: "iana", value: "America/New_York"},
		{name: "slashless iana Japan", value: "Japan"},
		{name: "slashless iana Turkey", value: "Turkey"},
		{name: "legacy us eastern", value: "US/Eastern"},
		{name: "iana EST as fixed offset", value: "EST"},
		{name: "posix EST5EDT", value: "EST5EDT"},
		{name: "unknown name", value: "Not/AZone", wantErr: "Not/AZone"},
		{name: "gibberish", value: "definitely not a zone", wantErr: "definitely not a zone"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.value.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestScheduleTimezone_ToServiceLocation(t *testing.T) {
	t.Parallel()

	t.Run("blank returns default", func(t *testing.T) {
		t.Parallel()

		loc := ScheduleTimezone("").ToServiceLocation()
		assert.False(t, loc.IsExplicit())
		assert.Same(t, model.DefaultScheduleTimezone, loc.ResolvedLocation())
	})

	t.Run("configured value resolves as explicit", func(t *testing.T) {
		t.Parallel()

		loc := ScheduleTimezone("America/New_York").ToServiceLocation()
		assert.True(t, loc.IsExplicit())
		assert.Equal(t, "America/New_York", loc.ResolvedLocation().String())
	})
}

func TestScheduleTimezone_ToRoutineLocation(t *testing.T) {
	t.Parallel()

	service := ScheduleTimezone("America/New_York").ToServiceLocation()

	t.Run("blank inherits service", func(t *testing.T) {
		t.Parallel()

		loc := ScheduleTimezone("").ToRoutineLocation(service)
		assert.False(t, loc.IsExplicit())
		assert.Equal(t, "America/New_York", loc.ResolvedLocation().String())
	})

	t.Run("override is explicit", func(t *testing.T) {
		t.Parallel()

		loc := ScheduleTimezone("UTC").ToRoutineLocation(service)
		assert.True(t, loc.IsExplicit())
		assert.Same(t, model.DefaultScheduleTimezone, loc.ResolvedLocation())
	})
}
