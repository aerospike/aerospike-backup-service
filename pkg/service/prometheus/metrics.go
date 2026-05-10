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
			Name: "aerospike_backup_service_backup_progress_ratio",
			Help: "Progress of backup processes as a ratio (0.0 to 1.0)",
		},
		[]string{labelRoutine, labelType},
	)

	backupRunningGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_backup_active",
			Help: "Number of backups currently running for the given routine and backup type (full/incremental)",
		},
		[]string{labelRoutine, labelType},
	)

	restoreRunningGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_restore_active",
			Help: "Number of restore processes running",
		},
		nil,
	)

	backupCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_backup_events_total",
			Help: "Total completed backup runs by routine, type, and outcome (success, failure, canceled, retry, skip)",
		},
		[]string{labelRoutine, labelType, labelOutcome},
	)

	restoreCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_restore_events_total",
			Help: "Total completed restore jobs by outcome (success, failure, canceled)",
		},
		[]string{labelOutcome},
	)

	backupDurationHist = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aerospike_backup_service_backup_duration_seconds",
			Help:    "Duration in seconds of finished backups by routine and type (full/incremental)",
			Buckets: prometheus.ExponentialBuckets(60, 1.5, 16), // 1 min to 10 hours
		},
		[]string{labelRoutine, labelType},
	)

	lastBackupTimestampGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_last_successful_backup_timestamp_seconds",
			Help: "Unix timestamp (seconds) of the last successful backup per routine",
		},
		[]string{labelRoutine, labelType},
	)

	// Deprecated metrics (for backwards compatibility).

	// Deprecated: use aerospike_backup_service_backup_progress_ratio instead.
	backupProgressDeprecated = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_backup_progress_pct",
			Help: "Progress of backup processes in percentage " +
				"(Deprecated: use aerospike_backup_service_backup_progress_ratio instead.)",
		},
		[]string{labelRoutine, labelType},
	)

	// Deprecated: use aerospike_backup_service_restore_active instead.
	restoreRunningGaugeDeprecated = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_restore_in_progress",
			Help: "Number of restore processes running " +
				"(Deprecated: use aerospike_backup_service_restore_active instead.)",
		},
		nil,
	)

	// Deprecated: use aerospike_backup_service_last_successful_backup_timestamp_seconds instead.
	lastBackupTimestampDeprecated = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_last_successful_backup_timestamp",
			Help: "Unix timestamp of the last successful backup per routine " +
				"(Deprecated: use aerospike_backup_service_last_successful_backup_timestamp_seconds instead.)",
		},
		[]string{labelRoutine, labelType},
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
		backupProgressDeprecated,
		restoreRunningGaugeDeprecated,
		lastBackupTimestampDeprecated,
	}

	prometheus.MustRegister(AllMetrics...)
}
