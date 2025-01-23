package service

import (
	"context"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// a counter metric for backup run number
	backupCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_runs_total",
			Help: "Successful backup runs counter",
		})
	// a counter metric for incremental backup run number
	incrBackupCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_incremental_runs_total",
			Help: "Successful incremental backup runs counter.",
		})
	// a counter metric for backup skip number
	backupSkippedCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_skip_total",
			Help: "Backup skip counter.",
		})
	// a counter metric for incremental backup skip number
	incrBackupSkippedCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_incremental_skip_total",
			Help: "Incremental backup skip counter.",
		})
	// a counter metric for backup failure number
	backupFailureCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_failure_total",
			Help: "Backup failure counter.",
		})
	// a counter metric for incremental backup failure number
	incrBackupFailureCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_incremental_failure_total",
			Help: "Incremental backup failure counter.",
		})
	// a gauge metric for full backup duration
	backupDurationGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_duration_millis",
			Help: "Full backup duration in milliseconds.",
		})
	// a gauge metric for incremental backup duration
	incrBackupDurationGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_incremental_duration_millis",
			Help: "Incremental backup duration in milliseconds.",
		})
	backupProgress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_backup_progress_pct",
			Help: "Progress of backup processes in percentage",
		},
		[]string{"routine", "type"},
	)
	restoreProgress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_backup_service_restore_progress_pct",
			Help: "Progress of restore processes in percentage",
		},
		[]string{"label"},
	)
)

func init() {
	prometheus.MustRegister(backupCounter)
	prometheus.MustRegister(incrBackupCounter)
	prometheus.MustRegister(backupSkippedCounter)
	prometheus.MustRegister(incrBackupSkippedCounter)
	prometheus.MustRegister(backupFailureCounter)
	prometheus.MustRegister(incrBackupFailureCounter)
	prometheus.MustRegister(backupDurationGauge)
	prometheus.MustRegister(incrBackupDurationGauge)
	prometheus.MustRegister(backupProgress, restoreProgress)
}

type MetricsCollector struct {
	mu       sync.Mutex
	backups  RunningBackupsRegistry
	restores *RestoreJobsHolder
}

// NewMetricsCollector creates a new MetricsCollector
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
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.collectBackupMetrics()
	mc.collectRestoreMetrics()
}

func (mc *MetricsCollector) collectBackupMetrics() {
	backupProgress.Reset()

	for _, routineName := range mc.backups.getRoutines() {
		currentStat := mc.backups.CurrentStat(routineName)
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
