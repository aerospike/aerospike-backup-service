package util

import (
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/util/attr"
	"github.com/reugn/go-quartz/quartz"
)

// IsCronFireTime returns true if the given timestamp exactly matches a scheduled fire time.
func IsCronFireTime(cronExpr string, t time.Time) bool {
	trigger, err := quartz.NewCronTrigger(cronExpr)
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
