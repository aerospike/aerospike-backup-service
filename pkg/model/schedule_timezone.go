package model

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // embed IANA zones; the Alpine image has no tzdata package
)

const (
	scheduleTimezoneUTC   = "utc"
	scheduleTimezoneLocal = "local"
	// DefaultScheduleTimezone is used when schedule-timezone is omitted.
	DefaultScheduleTimezone = "UTC"
)

// ParseScheduleTimezone turns a schedule-timezone config string into a location.
// Empty or "utc" (any case) is UTC. "local" (any case) is the host timezone.
// Any other value is an IANA name (case-sensitive). Abbreviations such as EST
// and POSIX TZ strings are rejected.
func ParseScheduleTimezone(s string) (*time.Location, error) {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "", scheduleTimezoneUTC:
		return time.UTC, nil
	case scheduleTimezoneLocal:
		return time.Local, nil
	default:
		if !strings.Contains(s, "/") {
			return nil, fmt.Errorf("invalid schedule-timezone %q: use %s, %s, or an IANA name such as America/New_York",
				s, scheduleTimezoneUTC, scheduleTimezoneLocal)
		}

		loc, err := time.LoadLocation(s)
		if err != nil {
			return nil, fmt.Errorf("invalid schedule-timezone %q: %w", s, err)
		}

		return loc, nil
	}
}

// ScheduleTimezonesEqual reports whether two schedule-timezone values resolve to
// the same location (empty and "UTC" are equal).
func ScheduleTimezonesEqual(a, b string) bool {
	locA, errA := ParseScheduleTimezone(a)
	locB, errB := ParseScheduleTimezone(b)
	if errA != nil || errB != nil {
		return a == b
	}

	return locA.String() == locB.String()
}
