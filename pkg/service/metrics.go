package service

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// a counter metric for backup run number
	backupCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_runs_total",
			Help: "Backup runs counter.",
		})
	// a counter metric for incremental backup run number
	incrBackupCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aerospike_backup_service_incremental_runs_total",
			Help: "Incremental backup runs counter.",
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
			Name: "aerospike_abs_backup_progress_pct",
			Help: "Progress of backup processes in percent",
		},
		[]string{"routine", "type"},
	)
	restoreProgress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aerospike_abs_restore_progress_pct",
			Help: "Progress of restore processes in percent",
		},
		[]string{"routine"},
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
	backupHandler BackupHandlerHolder
	jobsHolder    *RestoreJobsHolder
}

// NewMetricsCollector creates a new MetricsCollector
func NewMetricsCollector(bh BackupHandlerHolder, jh *RestoreJobsHolder) *MetricsCollector {
	return &MetricsCollector{
		backupHandler: bh,
		jobsHolder:    jh,
	}
}

func (mc *MetricsCollector) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
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
	mc.collectBackupMetrics()
	mc.collectRestoreMetrics()
}

func (mc *MetricsCollector) collectBackupMetrics() {
	backupProgress.Reset()

	for routineName, handler := range mc.backupHandler {
		currentStat := handler.GetCurrentStat()

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

// collectRestoreMetrics collects metrics from RestoreJobsHolder
func (mc *MetricsCollector) collectRestoreMetrics() {
	restoreProgress.Reset()

	mc.jobsHolder.Lock()
	defer mc.jobsHolder.Lock()

	for _, job := range mc.jobsHolder.jobs {
		restore := RestoreJobStatus(job).CurrentRestore
		if restore != nil {
			restoreProgress.WithLabelValues(job.label).Set(float64(restore.PercentageDone))
		}
	}
}
