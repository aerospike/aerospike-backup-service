package util

import (
	"time"

	"github.com/reugn/go-quartz/quartz"
)

// IsCronFireTime returns true if the given timestamp exactly matches a scheduled fire time.
func IsCronFireTime(cronExpr string, t time.Time) bool {
	trigger, _ := quartz.NewCronTrigger(cronExpr)
	fireTime, _ := trigger.NextFireTime(t.Add(-1 * time.Second).UnixNano())

	return fireTime == t.UnixNano()
}
