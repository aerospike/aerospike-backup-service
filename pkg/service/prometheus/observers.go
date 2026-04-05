package prometheus

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
)

// ObserveRestoreCompletion increments restores_total once per finished job.
// Maps model restore state to the same outcome strings as backups_total.
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
// For successful backups, startTime is recorded as the last successful backup timestamp.
func ObserveBackupEvent(
	routineName string,
	backupType model.BackupType,
	outcome Outcome,
	duration time.Duration,
	startTime time.Time,
) {
	labels := prometheus.Labels{
		"routine": routineName,
		"type":    string(backupType),
		"outcome": string(outcome),
	}
	backupCounters.With(labels).Inc()

	if outcome == OutcomeSuccess {
		ts := float64(startTime.Unix())
		lastBackupTimestamp.WithLabelValues(routineName, string(backupType)).Set(ts)
		lastBackupTimestampDeprecated.WithLabelValues(routineName, string(backupType)).Set(ts)
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
		lastBackupTimestampDeprecated.WithLabelValues(name, string(model.BackupTypeFull)).Set(t)
	}
	if lastRun.IncrementalBackupTime() != nil {
		t := float64(lastRun.IncrementalBackupTime().Unix())
		lastBackupTimestamp.WithLabelValues(name, string(model.BackupTypeIncremental)).Set(t)
		lastBackupTimestampDeprecated.WithLabelValues(name, string(model.BackupTypeIncremental)).Set(t)
	}
}
