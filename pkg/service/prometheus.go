package service

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

//nolint:lll
var (
	// A counter metric for backup run number.
	backupCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_runs_total",
			Help: "Successful backup runs counter (Deprecated, use `aerospike_backup_service_backup_events_total`)",
		})
	// A counter metric for incremental backup run number.
	incrBackupCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_incremental_runs_total",
			Help: "Successful incremental backup runs counter (Deprecated, use `aerospike_backup_service_backup_events_total`)",
		})
	// A counter metric for backup skip number.
	backupSkippedCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_skip_total",
			Help: "Full backup skip counter (Deprecated, use `aerospike_backup_service_backup_events_total`)",
		})
	// A counter metric for incremental backup skip number.
	incrBackupSkippedCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_incremental_skip_total",
			Help: "Incremental backup skip counter (Deprecated, use `aerospike_backup_service_backup_events_total`)",
		})
	// A counter metric for backup failure number.
	backupFailureCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_failure_total",
			Help: "Full backup failure counter (Deprecated, use `aerospike_backup_service_backup_events_total`)",
		})
	// A counter metric for incremental backup failure number.
	incrBackupFailureCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_incremental_failure_total",
			Help: "Incremental backup failure counter (Deprecated, use `aerospike_backup_service_backup_events_total`)",
		})
	// A gauge metric for full backup duration.
	backupDurationGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_duration_millis",
			Help: "Full backup duration in milliseconds (Deprecated, use `aerospike_backup_service_backup_duration_seconds`)",
		})
	// A gauge metric for incremental backup duration.
	incrBackupDurationGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_incremental_duration_millis",
			Help: "Incremental backup duration in milliseconds (Deprecated, use `aerospike_backup_service_backup_duration_seconds`)",
		})
	// A gauge metric for backup process, filter by name and type.
	backupProgress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_backup_progress_pct",
			Help: "Progress of backup processes in percentage",
		},
		[]string{"routine", "type"},
	)
	// A gauge metric for number of restore processes.
	restoreInProgress = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_restore_in_progress",
			Help: "Number of restore processes running",
		},
	)

	// A counter metric for backup job events.
	// Labels:
	//   - routine: name of the backup routine, e.g., "daily-ns1"
	//   - type: "full" or "incremental"
	//   - outcome: one of "success", "failure", "skip" or "retry"
	backupCounters = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_backup_events_total",
			Help: "Backup service job events by routine, type (full/incremental), and outcome (success, failure, retry, skip)",
		},
		[]string{"routine", "type", "outcome"},
	)

	// A histogram metric for backup job durations (in seconds).
	// Labels:
	//   - routine: name of the backup routine, e.g., "daily-ns1"
	//   - type: "full" or "incremental"
	backupDurations = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aerospike_backup_service_backup_duration_seconds",
			Help:    "Duration in seconds of finished backups by routine and type (full/incremental)",
			Buckets: prometheus.ExponentialBuckets(60, 1.5, 16), // 1 min to 10 hours
		},
		[]string{"routine", "type"},
	)
	// A gauge metric for unix timestamp of the last successful backup per routine.
	lastBackupTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_last_successful_backup_timestamp",
			Help: "Unix timestamp of the last successful backup per routine",
		},
		[]string{"routine"},
	)
)

var AllMetrics []prometheus.Collector

func init() {
	AllMetrics = []prometheus.Collector{
		backupCounter,
		incrBackupCounter,
		backupSkippedCounter,
		incrBackupSkippedCounter,
		backupFailureCounter,
		incrBackupFailureCounter,
		backupDurationGauge,
		incrBackupDurationGauge,
		backupProgress,
		restoreInProgress,
		backupCounters,
		backupDurations,
		lastBackupTimestamp,
	}

	prometheus.MustRegister(AllMetrics...)
}

type MetricsCollector struct {
	mu       sync.Mutex
	backups  RunningBackupsRegistry
	restores *RestoreJobsHolder
}

// NewMetricsCollector creates a new MetricsCollector.
func NewMetricsCollector(bh RunningBackupsRegistry, jh *RestoreJobsHolder) *MetricsCollector {
	return &MetricsCollector{
		backups:  bh,
		restores: jh,
	}
}

func (mc *MetricsCollector) Start(ctx context.Context, duration time.Duration) {
	ticker := time.NewTicker(duration)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mc.collectMetrics()
			}
		}
	}()
}

func (mc *MetricsCollector) collectMetrics() {
	if !mc.mu.TryLock() {
		return // Previous collection still ongoing.
	}
	defer mc.mu.Unlock()

	mc.collectBackupMetrics()
	mc.collectRestoreMetrics()
}

func (mc *MetricsCollector) collectBackupMetrics() {
	runningState := mc.backups.GetRunningState() // Collecting routines states can take some time.
	backupProgress.Reset()

	for routineName, currentStat := range runningState {
		// Update Full backup metric if running
		if currentStat.Full != nil {
			backupProgress.WithLabelValues(routineName, string(jobTypeFull)).
				Set(float64(currentStat.Full.PercentageDone))
		}

		// Update Incremental backup metric if running
		if currentStat.Incremental != nil {
			backupProgress.WithLabelValues(routineName, string(jobTypeIncremental)).
				Set(float64(currentStat.Incremental.PercentageDone))
		}
	}
}

func (mc *MetricsCollector) collectRestoreMetrics() {
	restoreInProgress.Set(float64(mc.restores.Size()))
}

type BackupOutcome string

const (
	BackupOutcomeSuccess BackupOutcome = "success"
	BackupOutcomeFailure BackupOutcome = "failure"
	BackupOutcomeRetry   BackupOutcome = "retry"
	BackupOutcomeSkip    BackupOutcome = "skip"
)

func observeBackupEvent(routineName string, backupType jobType, outcome BackupOutcome, duration time.Duration) {
	backupCounters.With(prometheus.Labels{
		"routine": routineName,
		"type":    string(backupType),
		"outcome": string(outcome),
	}).Inc()

	if duration > 0 {
		backupDurations.With(prometheus.Labels{
			"routine": routineName,
			"type":    string(backupType),
		}).Observe(duration.Seconds())
	}

	// update deprecated counters
	switch outcome {
	case BackupOutcomeSuccess:
		if backupType == jobTypeFull {
			backupCounter.Inc()
			backupDurationGauge.Set(float64(duration.Milliseconds()))
		} else {
			incrBackupCounter.Inc()
			incrBackupDurationGauge.Set(float64(duration.Milliseconds()))
		}
	case BackupOutcomeFailure:
		if backupType == jobTypeFull {
			backupFailureCounter.Inc()
		} else {
			incrBackupFailureCounter.Inc()
		}
	case BackupOutcomeSkip:
		if backupType == jobTypeFull {
			backupSkippedCounter.Inc()
		} else {
			incrBackupSkippedCounter.Inc()
		}
	case BackupOutcomeRetry:
		// No deprecated counter for retry.
	}
}
