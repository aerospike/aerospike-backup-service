package model

import (
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
// Values are populated by the DTO layer; the model never parses timezone strings.
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

// NewServiceLocation builds a service-level Location from a value already
// resolved by the DTO layer. A nil resolved timezone means "unset", producing
// the default source (UTC).
func NewServiceLocation(configured string, resolved *time.Location) Location {
	if resolved == nil {
		return Location{Configured: configured, Source: LocationSourceDefault}
	}

	return Location{
		resolved:   resolved,
		Configured: configured,
		Source:     LocationSourceService,
	}
}

// NewRoutineLocation builds a routine-level Location from a value already
// resolved by the DTO layer. When resolved is nil, the routine inherits the
// service Location (both resolved timezone and source).
func NewRoutineLocation(configured string, resolved *time.Location, service Location) Location {
	if resolved == nil {
		return Location{
			resolved:   service.resolved,
			Configured: configured,
			Source:     service.Source,
		}
	}

	return Location{
		resolved:   resolved,
		Configured: configured,
		Source:     LocationSourceRoutine,
	}
}

// ResolveTimezone turns a schedule-timezone configuration string into a
// *time.Location. Empty (or whitespace-only) returns (nil, nil), meaning "no
// value configured at this level." UTC and Local are case-insensitive
// keywords; every other value is passed to time.LoadLocation.
//
// This is the single source of truth for timezone parsing; the DTO layer
// wraps it to enforce validation policy.
func ResolveTimezone(configured string) (*time.Location, error) {
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

	return time.LoadLocation(trimmed)
}
