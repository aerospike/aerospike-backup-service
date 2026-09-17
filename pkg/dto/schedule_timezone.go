package dto

import (
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// ScheduleTimezone is the raw schedule-timezone value as it appears in
// configuration. Empty means "inherit from parent" (routine inherits service,
// service falls back to UTC).
//
// UTC and Local keywords are case-insensitive; every other value is passed to
// time.LoadLocation, which accepts any IANA name embedded via time/tzdata.
// Prefer canonical Area/Location names (America/New_York) over legacy
// abbreviations (EST): the latter is a fixed UTC-5 zone with no DST, which is
// rarely what an operator writing "Eastern Time" intends.
type ScheduleTimezone string

// Validate returns an error when the value cannot be resolved to a timezone.
func (t ScheduleTimezone) Validate() error {
	if _, err := t.parse(); err != nil {
		return errValidationInvalidValue(
			"schedule-timezone", string(t),
			"UTC, Local, or a valid IANA timezone name",
		)
	}

	return nil
}

// ToServiceLocation resolves the service-level timezone.
// A blank value produces the default (UTC) Location.
func (t ScheduleTimezone) ToServiceLocation() model.Location {
	loc, _ := t.parse()
	return model.NewServiceLocation(string(t), loc)
}

// ToRoutineLocation resolves a routine-level timezone, inheriting from the
// service default when the routine value is blank.
func (t ScheduleTimezone) ToRoutineLocation(service model.Location) model.Location {
	loc, _ := t.parse()
	return model.NewRoutineLocation(string(t), loc, service)
}

func (t ScheduleTimezone) parse() (*time.Location, error) {
	loc, err := model.ResolveTimezone(string(t))
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q", string(t))
	}

	return loc, nil
}
