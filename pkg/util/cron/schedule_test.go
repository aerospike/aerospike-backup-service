package cron

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsFireTime(t *testing.T) {
	schedule := Schedule{Cron: "@daily", Location: time.UTC}

	assert.True(t, schedule.IsFireTime(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)))
	assert.False(t, schedule.IsFireTime(time.Date(2024, 1, 2, 1, 0, 0, 0, time.UTC)))
}

// TestIsFireTimeHonoursLocation pins the reason Cron and Location travel together:
// the same expression fires at a different instant once the timezone changes.
func TestIsFireTimeHonoursLocation(t *testing.T) {
	// Local midnight in UTC+3 is 21:00 UTC on the previous day.
	instant := time.Date(2024, 1, 1, 21, 0, 0, 0, time.UTC)

	eastOfUTC := Schedule{Cron: "@daily", Location: time.FixedZone("UTC+3", 3*60*60)}
	assert.True(t, eastOfUTC.IsFireTime(instant))

	utc := Schedule{Cron: "@daily", Location: time.UTC}
	assert.False(t, utc.IsFireTime(instant))
}

func TestNextTrigger(t *testing.T) {
	schedule := Schedule{Cron: "@daily", Location: time.UTC}

	next, err := schedule.NextTrigger()

	require.NoError(t, err)
	assert.True(t, next.After(time.Now()), "next trigger %s should be in the future", next)
}

func TestNextTrigger_Invalid(t *testing.T) {
	tests := map[string]Schedule{
		"malformed cron":   {Cron: "not a cron", Location: time.UTC},
		"missing cron":     {Location: time.UTC},
		"missing location": {Cron: "@daily"},
	}

	for name, schedule := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := schedule.NextTrigger()

			require.Error(t, err)
		})
	}
}

func TestIsFireTime_SecondPrecision(t *testing.T) {
	type testCase struct {
		schedule Schedule
		at       time.Time
		want     bool
	}

	tests := map[string]testCase{
		"matches the configured second": {
			schedule: Schedule{Cron: "15 30 14 * * *", Location: time.UTC},
			at:       time.Date(2025, 7, 20, 14, 30, 15, 0, time.UTC),
			want:     true,
		},
		"matches hourly on the minute": {
			schedule: Schedule{Cron: "0 3 * * * *", Location: time.UTC},
			at:       time.Date(2025, 7, 20, 12, 3, 0, 0, time.UTC),
			want:     true,
		},
		"misses the wrong minute": {
			schedule: Schedule{Cron: "0 3 * * * *", Location: time.UTC},
			at:       time.Date(2025, 7, 20, 12, 4, 0, 0, time.UTC),
			want:     false,
		},
		"misses on sub-second drift": {
			schedule: Schedule{Cron: "0 0 * * * *", Location: time.UTC},
			at:       time.Date(2025, 7, 20, 15, 0, 0, 999_000_000, time.UTC),
			want:     false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.schedule.IsFireTime(tt.at))
		})
	}
}

func TestIsFireTime_IANAZone(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	daily := Schedule{Cron: "@daily", Location: loc}
	twoAM := Schedule{Cron: "0 0 2 * * ?", Location: loc}

	utcMidnight := time.Date(2025, 7, 20, 0, 0, 0, 0, time.UTC)

	assert.True(t, daily.IsFireTime(time.Date(2025, 7, 20, 0, 0, 0, 0, loc)))
	assert.False(t, daily.IsFireTime(utcMidnight))
	assert.True(t, twoAM.IsFireTime(time.Date(2025, 7, 20, 2, 0, 0, 0, loc)))
	assert.False(t, twoAM.IsFireTime(utcMidnight))
}

func TestNextTrigger_IANAZone(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	next, err := Schedule{Cron: "@daily", Location: loc}.NextTrigger()

	require.NoError(t, err)
	inLoc := next.In(loc)
	assert.Equal(t, 0, inLoc.Hour())
	assert.Equal(t, 0, inLoc.Minute())
	assert.Equal(t, 0, inLoc.Second())
}
