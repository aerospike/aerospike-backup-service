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
func ObserveBackupEvent(
	routineName string,
	backupType model.BackupType,
	outcome Outcome,
	duration time.Duration,
) {
	labels := prometheus.Labels{
		labelRoutine: routineName,
		labelType:    string(backupType),
		labelOutcome: string(outcome),
	}
	backupCounter.With(labels).Inc()

	// last_successful_backup_timestamp is not set here. On success, BackupSucceeded
	// triggers an async storage scan that updates the gauge via SetLastBackupTimestamp.

	if duration > 0 {
		backupDurationHist.With(prometheus.Labels{
			labelRoutine: routineName,
			labelType:    string(backupType),
		}).Observe(duration.Seconds())
	}

	updateDeprecatedBackupCounters(backupType, outcome, duration)
}

func updateDeprecatedBackupCounters(backupType model.BackupType, outcome Outcome, duration time.Duration) {
	switch outcome {
	case OutcomeSuccess:
		if backupType == model.BackupTypeFull {
			backupRunsTotalDeprecated.WithLabelValues().Inc()
			backupDurationMillisDeprecated.WithLabelValues().Set(float64(duration.Milliseconds()))
		} else {
			incrBackupRunsTotalDeprecated.WithLabelValues().Inc()
			incrBackupDurationMillisDeprecated.WithLabelValues().Set(float64(duration.Milliseconds()))
		}
	case OutcomeFailure:
		if backupType == model.BackupTypeFull {
			backupFailureTotalDeprecated.WithLabelValues().Inc()
		} else {
			incrBackupFailureTotalDeprecated.WithLabelValues().Inc()
		}
	case OutcomeSkip:
		if backupType == model.BackupTypeFull {
			backupSkipTotalDeprecated.WithLabelValues().Inc()
		} else {
			incrBackupSkipTotalDeprecated.WithLabelValues().Inc()
		}
	case OutcomeRetry, OutcomeCanceled:
		// No deprecated counters for retry or canceled.
	}
}

func SetLastBackupTimestamp(name string, lastRun *model.BackupTime) {
	if lastRun.FullBackupTime() != nil {
		t := float64(lastRun.FullBackupTime().Unix())
		lastBackupTimestampGauge.WithLabelValues(name, string(model.BackupTypeFull)).Set(t)
	}
	if lastRun.IncrementalBackupTime() != nil {
		t := float64(lastRun.IncrementalBackupTime().Unix())
		lastBackupTimestampGauge.WithLabelValues(name, string(model.BackupTypeIncremental)).Set(t)
	}
}
