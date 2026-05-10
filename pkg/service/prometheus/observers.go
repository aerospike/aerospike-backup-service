package prometheus

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
)

// ObserveRestoreCompletion increments restores_total once per finished job.
func ObserveRestoreCompletion(status model.RestoreState) {
	switch status {
	case model.RestoreSuccess:
		restoreCounter.WithLabelValues(string(OutcomeSuccess)).Inc()
	case model.RestoreFailure:
		restoreCounter.WithLabelValues(string(OutcomeFailure)).Inc()
	case model.RestoreCanceled:
		restoreCounter.WithLabelValues(string(OutcomeCanceled)).Inc()
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
		labelRoutine: routineName,
		labelType:    string(backupType),
		labelOutcome: string(outcome),
	}
	backupCounter.With(labels).Inc()

	if outcome == OutcomeSuccess {
		ts := float64(startTime.Unix())
		lastBackupTimestampGauge.WithLabelValues(routineName, string(backupType)).Set(ts)
		lastBackupTimestampDeprecated.WithLabelValues(routineName, string(backupType)).Set(ts)
	}

	if duration > 0 {
		backupDurationHist.With(prometheus.Labels{
			labelRoutine: routineName,
			labelType:    string(backupType),
		}).Observe(duration.Seconds())
	}
}

func SetInitialLastBackup(name string, lastRun *model.BackupTime) {
	if lastRun.FullBackupTime() != nil {
		t := float64(lastRun.FullBackupTime().Unix())
		lastBackupTimestampGauge.WithLabelValues(name, string(model.BackupTypeFull)).Set(t)
		lastBackupTimestampDeprecated.WithLabelValues(name, string(model.BackupTypeFull)).Set(t)
	}
	if lastRun.IncrementalBackupTime() != nil {
		t := float64(lastRun.IncrementalBackupTime().Unix())
		lastBackupTimestampGauge.WithLabelValues(name, string(model.BackupTypeIncremental)).Set(t)
		lastBackupTimestampDeprecated.WithLabelValues(name, string(model.BackupTypeIncremental)).Set(t)
	}
}
