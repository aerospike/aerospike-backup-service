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
	// LocationSourceDefault means the timezone fell back to DefaultScheduleTimezone.
	LocationSourceDefault LocationSource = "utc"
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

// ParseTimezone resolves a schedule-timezone config value.
// Empty returns nil.
// UTC and Local keywords are case-insensitive; IANA names are case-sensitive. Abbreviations such as EST are rejected.
func ParseTimezone(configured string) (*time.Location, error) {
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
	// Safe to ignore errors: dto validation uses ParseTimezone first.
	resolved, _ := ParseTimezone(configured)

	return Location{
		resolved:   resolved,
		Configured: configured,
		Source:     locationSource(configured, LocationSourceService, LocationSourceDefault),
	}
}

// NewRoutineLocation builds a routine-level Location.
// When configured is blank, the routine inherits service's resolved timezone and source.
func NewRoutineLocation(configured string, service Location) Location {
	if strings.TrimSpace(configured) == "" {
		return Location{
			resolved:   service.resolved,
			Configured: configured,
			Source:     locationSource(configured, LocationSourceRoutine, service.Source),
		}
	}

	// Safe to ignore errors: dto validation uses ParseTimezone first.
	resolved, _ := ParseTimezone(configured)

	return Location{
		resolved:   resolved,
		Configured: configured,
		Source:     locationSource(configured, LocationSourceRoutine, LocationSourceDefault),
	}
}

// locationSource returns the source for this configuration level.
// When configured is blank, inherited is used; if inherited is also blank, UTC is used.
func locationSource(configuredTZ string, explicit, inherited LocationSource) LocationSource {
	if strings.TrimSpace(configuredTZ) != "" {
		return explicit
	}

	return inherited
}
