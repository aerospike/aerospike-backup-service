package model

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // embed IANA zones; the Alpine image has no tzdata package
)

// LocationSource identifies which configuration level supplied the resolved timezone.
type LocationSource string

const (
	// LocationSourceUTC means the timezone fell back to DefaultScheduleTimezone.
	LocationSourceUTC LocationSource = "utc"
	// LocationSourceService means the timezone came from service.backup.schedule-timezone.
	LocationSourceService LocationSource = "service"
	// LocationSourceRoutine means the timezone came from a routine's schedule-timezone.
	LocationSourceRoutine LocationSource = "routine"
)

// Location pairs a resolved timezone with its configured value and resolution source.
type Location struct {
	// resolved is the timezone used for evaluating cron expressions.
	// Nil means use DefaultScheduleTimezone via ResolvedLocation().
	resolved *time.Location
	// Configured is the schedule-timezone as configured at this level.
	// This value is not used in business logic.
	Configured string
	// Source records how resolved was chosen.
	Source LocationSource
}

// ResolvedLocation returns the resolved timezone, defaulting to DefaultScheduleTimezone.
func (l Location) ResolvedLocation() *time.Location {
	if l.resolved == nil {
		return DefaultScheduleTimezone
	}

	return l.resolved
}

// ParseScheduleTimezone resolves a schedule-timezone config value.
// Empty returns nil.
// UTC and Local keywords are case-insensitive; IANA names are case-sensitive. Abbreviations such as EST are rejected.
func ParseScheduleTimezone(configured string) (*time.Location, error) {
	trimmed := strings.TrimSpace(configured)
	if trimmed == "" {
		return nil, nil
	}

	switch strings.ToLower(trimmed) {
	case "utc":
		return time.UTC, nil
	case "local":
		return time.LoadLocation("Local")
	}

	if !strings.Contains(trimmed, "/") {
		return nil, fmt.Errorf(
			"invalid schedule-timezone %q: use UTC, Local, or an IANA name such as America/New_York",
			configured,
		)
	}

	return time.LoadLocation(trimmed)
}

// NewServiceLocation builds a service-level Location.
func NewServiceLocation(configured string) Location {
	// Safe to ignore errors: dto validation uses ParseScheduleTimezone first.
	resolved, _ := ParseScheduleTimezone(configured)

	source := LocationSourceUTC
	if strings.TrimSpace(configured) != "" {
		source = LocationSourceService
	}

	return Location{
		resolved:   resolved,
		Configured: configured,
		Source:     source,
	}
}

// NewRoutineLocation builds a routine-level Location.
// When configured is blank, the routine inherits service's resolved timezone and source.
func NewRoutineLocation(configured string, service Location) Location {
	if strings.TrimSpace(configured) == "" {
		source := service.Source
		if source == "" {
			source = LocationSourceUTC
		}

		return Location{
			resolved:   service.resolved,
			Configured: configured,
			Source:     source,
		}
	}

	// Safe to ignore errors: dto validation uses ParseScheduleTimezone first.
	resolved, _ := ParseScheduleTimezone(configured)
	return Location{
		resolved:   resolved,
		Configured: configured,
		Source:     LocationSourceRoutine,
	}
}
