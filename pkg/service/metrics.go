package service

import (
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
		[]string{"routine"},
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
