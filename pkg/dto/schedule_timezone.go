package dto

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // embed IANA zones; the Alpine image has no tzdata package
)

const (
	scheduleTimezoneUTC       = "utc"
	scheduleTimezoneLocal     = "local"
	scheduleTimezoneUTCName   = "UTC"
	scheduleTimezoneLocalName = "Local"
)

// parseTimezone turns a schedule-timezone config string into a location.
// Empty or "utc" (any case) is UTC. "local" (any case) is the host timezone.
// Any other value is an IANA name (case-sensitive). Abbreviations such as EST
// and POSIX TZ strings are rejected.
func parseTimezone(s string) (*time.Location, error) {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "", scheduleTimezoneUTC:
		return time.UTC, nil
	case scheduleTimezoneLocal:
		return time.Local, nil
	default:
		if !strings.Contains(s, "/") {
			return nil, fmt.Errorf(
				"invalid schedule-timezone %q: use %s, %s, or an IANA name such as America/New_York",
				s,
				scheduleTimezoneUTCName,
				scheduleTimezoneLocalName,
			)
		}

		return time.LoadLocation(s)
	}
}
