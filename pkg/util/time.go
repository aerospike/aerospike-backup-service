package util

import (
	"github.com/reugn/go-quartz/quartz"
	"time"
)

func PreviousCron(t time.Time, cronSpec string) time.Time {
	trigger, _ := quartz.NewCronTrigger(cronSpec)
	next := func(t time.Time) time.Time {
		fireTime, _ := trigger.NextFireTime(t.UnixNano())
		return time.Unix(0, fireTime)
	}

	return previous(t, next)
}

// previous returns the previous fireTime of a given time.
func previous(t time.Time, next func(time.Time) time.Time) time.Time {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Use binary search between start and end (input time)
	end := t
	for end.Sub(start).Hours() > 1 {
		mid := start.Add(end.Sub(start) / 2)

		nextMid := next(mid)

		if nextMid.After(t) {
			end = mid
		} else {
			start = mid
		}
	}

	return start
}
