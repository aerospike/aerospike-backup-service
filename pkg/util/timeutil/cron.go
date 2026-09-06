package timeutil

import (
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/reugn/go-quartz/quartz"
)

// IsCronFireTime returns true if the given timestamp exactly matches a scheduled fire time.
func IsCronFireTime(cronExpr string, t time.Time, loc *time.Location) bool {
	trigger, err := quartz.NewCronTriggerWithLoc(cronExpr, loc)
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

// NextTrigger returns the next scheduled fire time for the given cron expression.
func NextTrigger(cron string, loc *time.Location) (time.Time, error) {
	trigger, err := quartz.NewCronTriggerWithLoc(cron, loc)
	if err != nil {
		return time.Time{}, err
	}

	fireTime, err := trigger.NextFireTime(time.Now().UnixNano())
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(0, fireTime), nil
}
