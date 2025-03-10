package util

import (
	"testing"
	"time"

	"github.com/reugn/go-quartz/quartz"
)

// Test implementations of next functions.
func nextMidnight(t time.Time) time.Time {
	// Return the next midnight
	midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	if !midnight.After(t) {
		midnight = midnight.AddDate(0, 0, 1)
	}
	return midnight
}

func nextFirstOfMonth(t time.Time) time.Time {
	// Return the next 1st of the month
	firstOfMonth := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	if !firstOfMonth.After(t) {
		if t.Month() == time.December {
			firstOfMonth = time.Date(t.Year()+1, time.January, 1, 0, 0, 0, 0, t.Location())
		} else {
			firstOfMonth = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
		}
	}
	return firstOfMonth
}

func nextHourly(t time.Time) time.Time {
	// Return the next hour
	nextHour := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	if !nextHour.After(t) {
		nextHour = nextHour.Add(time.Hour)
	}
	return nextHour
}

func TestPrevious(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	testCases := []struct {
		name     string
		nextFunc func(time.Time) time.Time
	}{
		{
			name:     "previous midnight",
			nextFunc: nextMidnight,
		},
		{
			name:     "Exactly at midnight",
			nextFunc: nextMidnight,
		},
		{
			name:     "previous first of month",
			nextFunc: nextFirstOfMonth,
		},
		{
			name:     "Exactly at first of month",
			nextFunc: nextFirstOfMonth,
		},
		{
			name:     "previous hour",
			nextFunc: nextHourly,
		},
		{
			name: "cron",
			nextFunc: func(t time.Time) time.Time {
				trigger, _ := quartz.NewCronTrigger("*/10 * * * * *")
				fireTime, _ := trigger.NextFireTime(t.UnixNano())
				return time.Unix(0, fireTime)
			},
		},
	}

	input := time.Date(2025, 6, 1, 0, 1, 0, 0, loc)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := previous(input, tc.nextFunc)

			// The key requirement: next(result) should be <= input
			nextAfterResult := tc.nextFunc(result)
			if nextAfterResult.After(input) {
				t.Errorf("next(result) = %v is after input %v",
					nextAfterResult, input)
			}

			margin := input.Sub(nextAfterResult)
			if margin > errorMargin {
				t.Errorf("error margin between result and next(result) is too big (%v)", margin)
			}
		})
	}
}
