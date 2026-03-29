package prometheus

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
)

// ObserveRestoreCompletion increments restore_events_total once per finished job.
// Maps model restore state to the same outcome strings as backup_events_total.
func ObserveRestoreCompletion(status model.RestoreState) {
	switch status {
	case model.RestoreDone:
		restoreEventsTotal.WithLabelValues(string(OutcomeSuccess)).Inc()
	case model.RestoreFailed:
		restoreEventsTotal.WithLabelValues(string(OutcomeFailure)).Inc()
	case model.RestoreCanceled:
		restoreEventsTotal.WithLabelValues(string(OutcomeCanceled)).Inc()
	default:
		// running or unknown — should not happen after finish()
	}
}

// ObserveBackupEvent updates Prometheus backup counters/histograms.
func ObserveBackupEvent(
	routineName string,
	backupType model.BackupType,
	outcome Outcome,
	duration time.Duration,
) {
	backupCounters.With(prometheus.Labels{
		"routine": routineName,
		"type":    string(backupType),
		"outcome": string(outcome),
	}).Inc()

	if outcome == OutcomeSuccess {
		lastBackupTimestamp.WithLabelValues(routineName, string(backupType)).Set(float64(time.Now().Unix()))
	}

	if duration > 0 {
		backupDurations.With(prometheus.Labels{
			"routine": routineName,
			"type":    string(backupType),
		}).Observe(duration.Seconds())
	}
}

func SetInitialLastBackup(name string, lastRun *model.BackupTime) {
	if lastRun.FullBackupTime() != nil {
		t := float64(lastRun.FullBackupTime().Unix())
		lastBackupTimestamp.WithLabelValues(name, string(model.BackupTypeFull)).Set(t)
	}
	if lastRun.IncrementalBackupTime() != nil {
		t := float64(lastRun.IncrementalBackupTime().Unix())
		lastBackupTimestamp.WithLabelValues(name, string(model.BackupTypeIncremental)).Set(t)
	}
}
