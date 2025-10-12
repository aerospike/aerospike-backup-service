package timeutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheckCron_MatchingDaily(t *testing.T) {
	cronExpr := "@daily" // every day
	testTime := time.Date(2025, 7, 20, 0, 0, 0, 0, time.UTC)

	assert.True(t, IsCronFireTime(cronExpr, testTime), "Expected cron to match at 00:00:00")
}

func TestCheckCron_MatchingTimeWithSeconds(t *testing.T) {
	cronExpr := "0 3 * * * *" // every hour at mm:03:00
	testTime := time.Date(2025, 7, 20, 12, 3, 0, 0, time.UTC)

	assert.True(t, IsCronFireTime(cronExpr, testTime), "Expected cron to match at 12:03:00")
}

func TestCheckCron_NonMatchingTimeWithSeconds(t *testing.T) {
	cronExpr := "0 3 * * * *"
	testTime := time.Date(2025, 7, 20, 12, 4, 0, 0, time.UTC)

	assert.False(t, IsCronFireTime(cronExpr, testTime), "Expected cron not to match at 12:04:00")
}

func TestCheckCron_MatchOnSeconds(t *testing.T) {
	cronExpr := "15 30 14 * * *" // Every day at 14:30:15
	testTime := time.Date(2025, 7, 20, 14, 30, 15, 0, time.UTC)

	assert.True(t, IsCronFireTime(cronExpr, testTime), "Expected cron to match at 14:30:15")
}

func TestCheckCron_SubSecondMismatch(t *testing.T) {
	cronExpr := "0 0 * * * *"                                           // every hour on the hour
	testTime := time.Date(2025, 7, 20, 15, 0, 0, 999_000_000, time.UTC) // 15:00:00.999

	// This should not match, as fire time is 15:00:00.000
	assert.False(t, IsCronFireTime(cronExpr, testTime), "Expected false due to nanosecond mismatch")
}
