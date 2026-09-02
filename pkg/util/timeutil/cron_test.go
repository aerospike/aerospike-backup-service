package timeutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckCron_MatchingDaily(t *testing.T) {
	cronExpr := "@daily" // every day
	testTime := time.Date(2025, 7, 20, 0, 0, 0, 0, time.UTC)

	assert.True(t, IsCronFireTime(cronExpr, testTime, time.UTC), "Expected cron to match at 00:00:00")
}

func TestCheckCron_MatchingTimeWithSeconds(t *testing.T) {
	cronExpr := "0 3 * * * *" // every hour at mm:03:00
	testTime := time.Date(2025, 7, 20, 12, 3, 0, 0, time.UTC)

	assert.True(t, IsCronFireTime(cronExpr, testTime, time.UTC), "Expected cron to match at 12:03:00")
}

func TestCheckCron_NonMatchingTimeWithSeconds(t *testing.T) {
	cronExpr := "0 3 * * * *"
	testTime := time.Date(2025, 7, 20, 12, 4, 0, 0, time.UTC)

	assert.False(t, IsCronFireTime(cronExpr, testTime, time.UTC), "Expected cron not to match at 12:04:00")
}

func TestCheckCron_MatchOnSeconds(t *testing.T) {
	cronExpr := "15 30 14 * * *" // Every day at 14:30:15
	testTime := time.Date(2025, 7, 20, 14, 30, 15, 0, time.UTC)

	assert.True(t, IsCronFireTime(cronExpr, testTime, time.UTC), "Expected cron to match at 14:30:15")
}

func TestCheckCron_SubSecondMismatch(t *testing.T) {
	cronExpr := "0 0 * * * *"                                           // every hour on the hour
	testTime := time.Date(2025, 7, 20, 15, 0, 0, 999_000_000, time.UTC) // 15:00:00.999

	// This should not match, as fire time is 15:00:00.000
	assert.False(t, IsCronFireTime(cronExpr, testTime, time.UTC), "Expected false due to nanosecond mismatch")
}

func TestNextTrigger(t *testing.T) {
	next, err := NextTrigger("@daily", time.UTC)
	require.NoError(t, err)
	assert.True(t, next.After(time.Now()))

	nextHour, err := NextTrigger("0 0 * * * *", time.UTC)
	require.NoError(t, err)
	assert.True(t, nextHour.After(time.Now()))
}

func TestCheckCron_DailyInAmericaNewYork(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	localMidnight := time.Date(2025, 7, 20, 0, 0, 0, 0, loc)
	utcMidnight := time.Date(2025, 7, 20, 0, 0, 0, 0, time.UTC)
	twoAM := time.Date(2025, 7, 20, 2, 0, 0, 0, loc)

	assert.True(t, IsCronFireTime("@daily", localMidnight, loc))
	assert.False(t, IsCronFireTime("@daily", utcMidnight, loc))
	assert.True(t, IsCronFireTime("0 0 2 * * ?", twoAM, loc))
	assert.False(t, IsCronFireTime("0 0 2 * * ?", utcMidnight, loc))
}

func TestNextTrigger_AmericaNewYorkDaily(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	next, err := NextTrigger("@daily", loc)
	require.NoError(t, err)

	inLoc := next.In(loc)
	assert.Equal(t, 0, inLoc.Hour())
	assert.Equal(t, 0, inLoc.Minute())
	assert.Equal(t, 0, inLoc.Second())
}
