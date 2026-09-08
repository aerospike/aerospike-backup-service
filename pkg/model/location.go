package model

import (
	"strings"
	"time"
	_ "time/tzdata" // embed IANA zones; the Alpine image has no tzdata package
)

// Location pairs a resolved timezone with its configured value.
// Values are populated by the DTO layer; the model never parses timezone strings.
//
// A routine's Location may inherit the service's resolved timezone. In that
// case the routine's own configured string is empty even though the routine
// still reports a non-default resolved timezone via ResolvedLocation.
type Location struct {
	// resolved is the timezone used for evaluating cron expressions.
	// Nil means fall back to DefaultScheduleTimezone via ResolvedLocation.
	resolved *time.Location
	// configured is the schedule-timezone string as it appeared at this
	// level of the config, preserved verbatim so read-modify-write of the
	// YAML does not rewrite the operator's spelling.
	Configured string
}

// ResolvedLocation returns the resolved timezone, defaulting to DefaultScheduleTimezone.
func (l Location) ResolvedLocation() *time.Location {
	if l.resolved == nil {
		return DefaultScheduleTimezone
	}

	return l.resolved
}

// IsExplicit reports whether this level supplied its own resolved timezone,
// as opposed to inheriting from a parent or falling back to the default.
func (l Location) IsExplicit() bool {
	return l.resolved != nil && strings.TrimSpace(l.Configured) != ""
}

// NewServiceLocation builds a service-level Location from a value already
// resolved by the DTO layer. A nil resolved timezone means "unset", producing
// a Location that reports UTC via ResolvedLocation.
func NewServiceLocation(configured string, resolved *time.Location) Location {
	return Location{resolved: resolved, Configured: configured}
}

// NewRoutineLocation builds a routine-level Location from a value already
// resolved by the DTO layer. When resolved is nil, the routine inherits the
// service's resolved timezone; the routine's own configured string is kept
// as-is (typically empty) so round-tripping preserves the operator's intent.
func NewRoutineLocation(configured string, resolved *time.Location, service Location) Location {
	if resolved == nil {
		return Location{resolved: service.resolved, Configured: configured}
	}

	return Location{resolved: resolved, Configured: configured}
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
