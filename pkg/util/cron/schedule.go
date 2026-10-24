// Package cron evaluates cron expressions against an explicit timezone.
package cron

import (
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/reugn/go-quartz/quartz"
)

// Schedule is a cron expression together with the timezone it is evaluated in.
// The two are meaningless apart — the same expression fires at different instants in
// different zones — so they travel as one value rather than as two parameters that a
// caller could pair up wrongly.
//
// A Schedule built without a Location reports an error from NextTrigger rather than
// silently evaluating in the wrong zone.
type Schedule struct {
	// Cron is the cron expression. Empty means no schedule is configured.
	Cron string
	// Location is the timezone the expression is evaluated in.
	Location *time.Location
}

func NewSchedule(cron string, location *time.Location) Schedule {
	return Schedule{
		Cron:     cron,
		Location: location,
	}
}

// NextTrigger returns the next instant at which the schedule fires.
func (s Schedule) NextTrigger() (time.Time, error) {
	trigger, err := s.trigger()
	if err != nil {
		return time.Time{}, err
	}

	fireTime, err := trigger.NextFireTime(time.Now().UnixNano())
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(0, fireTime), nil
}

// IsFireTime reports whether t is exactly a fire time of this schedule.
// An unparseable schedule fires at no instant: the failure is logged and reported as
// false, so callers that validate their expressions up front need no error path here.
func (s Schedule) IsFireTime(t time.Time) bool {
	trigger, err := s.trigger()
	if err != nil {
		slog.Error("Failed to parse cron expression", attr.Error(err))
		return false
	}

	fireTime, err := trigger.NextFireTime(t.Add(-1 * time.Second).UnixNano())
	if err != nil {
		slog.Error("Failed to get next fire time", attr.Error(err))
		return false
	}

	return fireTime == t.UnixNano()
}

// trigger builds the quartz trigger this schedule describes.
func (s Schedule) trigger() (*quartz.CronTrigger, error) {
	return quartz.NewCronTriggerWithLoc(s.Cron, s.Location)
}
