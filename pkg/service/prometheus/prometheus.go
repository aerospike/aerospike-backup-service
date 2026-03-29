package prometheus

import (
	"context"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
)

// Outcome is the outcome label on backup_events_total and restore_events_total.
type Outcome string

const (
	OutcomeSuccess  Outcome = "success"
	OutcomeFailure  Outcome = "failure"
	OutcomeCanceled Outcome = "canceled"
	OutcomeRetry    Outcome = "retry"
	OutcomeSkip     Outcome = "skip"
)

var (
	backupProgress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_backup_progress_pct",
			Help: "Progress of backup processes in percentage",
		},
		[]string{"routine", "type"},
	)
	// 1 while a (routine, type) backup slot is active; series removed when idle (Reset each scrape).
	backupRunningGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_backup_running",
			Help: "1 while the given routine and backup type (full/incremental) has a run in progress; " +
				"absent after scrape Reset when idle",
		},
		[]string{"routine", "type"},
	)
	// Number of restore jobs currently in running state (GaugeVec with no labels for readme/metrics extraction).
	restoreRunningGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_restore_in_progress",
			Help: "Number of restore processes running",
		},
		nil,
	)

	backupCounters = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_backup_events_total",
			Help: "Total completed backup runs by routine, type, and outcome (success, failure, canceled, retry, skip)",
		},
		[]string{"routine", "type", "outcome"},
	)

	restoreEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_restore_events_total",
			Help: "Total completed restore jobs by outcome (success, failure, canceled)",
		},
		[]string{"outcome"},
	)

	backupDurations = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aerospike_backup_service_backup_duration_seconds",
			Help:    "Duration in seconds of finished backups by routine and type (full/incremental)",
			Buckets: prometheus.ExponentialBuckets(60, 1.5, 16), // 1 min to 10 hours
		},
		[]string{"routine", "type"},
	)
	lastBackupTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_last_successful_backup_timestamp",
			Help: "Unix timestamp of the last successful backup per routine",
		},
		[]string{"routine", "type"},
	)
)

var AllMetrics []prometheus.Collector

func init() {
	AllMetrics = []prometheus.Collector{
		backupProgress,
		backupRunningGauge,
		restoreRunningGauge,
		backupCounters,
		restoreEventsTotal,
		backupDurations,
		lastBackupTimestamp,
	}

	prometheus.MustRegister(AllMetrics...)
}

// RunningBackupsRegistry defines the interface for managing running backups and their statuses.
type RunningBackupsRegistry interface {
	// GetRunningState returns statistics for all current backups.
	GetRunningState() map[string]model.RoutineState
	// Cancel stops all ongoing backups for a specific routine.
}

type RestoreJobsHolder interface {
	StatusCounts() map[model.RestoreState]int
}

type MetricsCollector struct {
	mu       sync.Mutex
	backups  RunningBackupsRegistry
	restores RestoreJobsHolder
}

// NewMetricsCollector creates a new MetricsCollector.
func NewMetricsCollector(bh RunningBackupsRegistry, jh RestoreJobsHolder) *MetricsCollector {
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
	runningState := mc.backups.GetRunningState()

	backupRunningGauge.Reset()
	for routineName, rs := range runningState {
		if rs.Full != nil {
			backupRunningGauge.WithLabelValues(routineName, string(model.BackupTypeFull)).Set(1)
		}
		if rs.Incremental != nil {
			backupRunningGauge.WithLabelValues(routineName, string(model.BackupTypeIncremental)).Set(1)
		}
	}

	backupProgress.Reset()

	for routineName, currentStat := range runningState {
		if currentStat.Full != nil {
			backupProgress.WithLabelValues(routineName, string(model.BackupTypeFull)).
				Set(float64(currentStat.Full.PercentageDone))
		}

		if currentStat.Incremental != nil {
			backupProgress.WithLabelValues(routineName, string(model.BackupTypeIncremental)).
				Set(float64(currentStat.Incremental.PercentageDone))
		}
	}
}

func (mc *MetricsCollector) collectRestoreMetrics() {
	counts := mc.restores.StatusCounts()
	restoreRunningGauge.WithLabelValues().Set(float64(counts[model.RestoreRunning]))
}

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
