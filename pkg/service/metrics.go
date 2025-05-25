package service

import (
	"context"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// A counter metric for backup run number.
	backupCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_runs_total",
			Help: "Successful backup runs counter",
		})
	// A counter metric for incremental backup run number.
	incrBackupCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_incremental_runs_total",
			Help: "Successful incremental backup runs counter",
		})
	// A counter metric for backup skip number.
	backupSkippedCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_skip_total",
			Help: "Full backup skip counter",
		})
	// A counter metric for incremental backup skip number.
	incrBackupSkippedCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_incremental_skip_total",
			Help: "Incremental backup skip counter",
		})
	// A counter metric for backup failure number.
	backupFailureCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_failure_total",
			Help: "Full backup failure counter",
		})
	// A counter metric for incremental backup failure number.
	incrBackupFailureCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_incremental_failure_total",
			Help: "Incremental backup failure counter",
		})
	// A gauge metric for full backup duration.
	backupDurationGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_duration_millis",
			Help: "Full backup duration in milliseconds",
		})
	// A gauge metric for incremental backup duration.
	incrBackupDurationGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_incremental_duration_millis",
			Help: "Incremental backup duration in milliseconds",
		})
	// A gauge metrics for backup process, filter by name and type.
	backupProgress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_backup_progress_pct",
			Help: "Progress of backup processes in percentage",
		},
		[]string{"routine", "type"},
	)
	// A gauge metrics for restore processes.
	restoreProgress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_restore_progress_pct",
			Help: "Progress of restore processes in percentage",
		},
		[]string{"label"},
	)

	// A counter metric for backup job events.
	// Labels:
	//   - routine: name of the backup routine, e.g., "daily-ns1"
	//   - type: "full" or "incremental"
	//   - outcome: one of "success", "failure", or "retry"
	backupJobEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_job_events_total",
			Help: "Backup service job events by routine, type, and outcome",
		},
		[]string{"routine", "type", "outcome"},
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
		restoreProgress,
		backupJobEvents,
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
			backupProgress.WithLabelValues(routineName, "Full").Set(float64(currentStat.Full.PercentageDone))
		}

		// Update Incremental backup metric if running
		if currentStat.Incremental != nil {
			backupProgress.WithLabelValues(routineName, "Incremental").Set(float64(currentStat.Incremental.PercentageDone))
		}
	}
}

func (mc *MetricsCollector) collectRestoreMetrics() {
	restoreProgress.Reset()

	mc.restores.Iterate(func(_ model.RestoreJobID, job *jobInfo) {
		restore := RestoreJobStatus(job).CurrentRestore // CurrentRestore exists only for running jobs
		if restore != nil {
			restoreProgress.WithLabelValues(job.label).Set(float64(restore.PercentageDone))
		}
	})
}
