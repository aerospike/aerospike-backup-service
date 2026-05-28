package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Outcome is the outcome label on backup and restore events.
type Outcome string

const (
	OutcomeSuccess  Outcome = "success"
	OutcomeFailure  Outcome = "failure"
	OutcomeCanceled Outcome = "canceled"
	OutcomeRetry    Outcome = "retry"
	OutcomeSkip     Outcome = "skip"

	labelRoutine = "routine"
	labelType    = "type"
	labelOutcome = "outcome"
)

var (
	backupProgressGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_backup_progress_pct",
			Help: "Progress of backup processes in percentage",
		},
		[]string{labelRoutine, labelType},
	)

	restoreRunningGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_restore_in_progress",
			Help: "Number of restore processes running",
		},
		nil,
	)

	backupRunningGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_backup_active",
			Help: "Number of backups currently running for the given routine and backup type (full/incremental)",
		},
		[]string{labelRoutine, labelType},
	)

	backupCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_backup_events_total",
			Help: "Backup service job events by routine, type (full/incremental), and outcome (success, failure, canceled, retry, skip)",
		},
		[]string{labelRoutine, labelType, labelOutcome},
	)

	backupDurationHist = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aerospike_backup_service_backup_duration_seconds",
			Help:    "Duration in seconds of finished backups by routine and type (full/incremental)",
			Buckets: prometheus.ExponentialBuckets(60, 1.5, 16), // 1 min to 10 hours
		},
		[]string{labelRoutine, labelType},
	)

	restoreCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_restore_events_total",
			Help: "Total completed restore jobs by outcome (success, failure, canceled)",
		},
		[]string{labelOutcome},
	)

	lastBackupTimestampGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_last_successful_backup_timestamp",
			Help: "Unix timestamp (seconds) of the last successful backup per routine",
		},
		[]string{labelRoutine, labelType},
	)

	// Deprecated metrics (for backwards compatibility).

	backupRunsTotalDeprecated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_runs_total",
			Help: "Successful backup runs counter " +
				"(Deprecated: use aerospike_backup_service_backup_events_total instead.)",
		},
		nil,
	)

	incrBackupRunsTotalDeprecated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_incremental_runs_total",
			Help: "Successful incremental backup runs counter " +
				"(Deprecated: use aerospike_backup_service_backup_events_total instead.)",
		},
		nil,
	)

	backupSkipTotalDeprecated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_skip_total",
			Help: "Full backup skip counter " +
				"(Deprecated: use aerospike_backup_service_backup_events_total instead.)",
		},
		nil,
	)

	incrBackupSkipTotalDeprecated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_incremental_skip_total",
			Help: "Incremental backup skip counter " +
				"(Deprecated: use aerospike_backup_service_backup_events_total instead.)",
		},
		nil,
	)

	backupFailureTotalDeprecated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_failure_total",
			Help: "Full backup failure counter " +
				"(Deprecated: use aerospike_backup_service_backup_events_total instead.)",
		},
		nil,
	)

	incrBackupFailureTotalDeprecated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_incremental_failure_total",
			Help: "Incremental backup failure counter " +
				"(Deprecated: use aerospike_backup_service_backup_events_total instead.)",
		},
		nil,
	)

	backupDurationMillisDeprecated = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_duration_millis",
			Help: "Full backup duration in milliseconds " +
				"(Deprecated: use aerospike_backup_service_backup_duration_seconds instead.)",
		},
		nil,
	)

	incrBackupDurationMillisDeprecated = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_incremental_duration_millis",
			Help: "Incremental backup duration in milliseconds " +
				"(Deprecated: use aerospike_backup_service_backup_duration_seconds instead.)",
		},
		nil,
	)
)

var AllMetrics []prometheus.Collector

func init() {
	AllMetrics = []prometheus.Collector{
		backupProgressGauge,
		backupRunningGauge,
		restoreRunningGauge,
		backupCounter,
		restoreCounter,
		backupDurationHist,
		lastBackupTimestampGauge,

		// Deprecated metrics
		backupRunsTotalDeprecated,
		incrBackupRunsTotalDeprecated,
		backupSkipTotalDeprecated,
		incrBackupSkipTotalDeprecated,
		backupFailureTotalDeprecated,
		incrBackupFailureTotalDeprecated,
		backupDurationMillisDeprecated,
		incrBackupDurationMillisDeprecated,
	}

	prometheus.MustRegister(AllMetrics...)
}
