package service

import (
	"github.com/prometheus/client_golang/prometheus"
)

// a counter metric for backup run number
var backupCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "backup_runs_total",
		Help: "Backup runs counter.",
	})

// a counter metric for incremental backup run number
var incrBackupCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "backup_incremental_runs_total",
		Help: "Incremental backup runs counter.",
	})

// a counter metric for backup skip number
var backupSkippedCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "backup_skip_total",
		Help: "Backup skip counter.",
	})

// a counter metric for incremental backup skip number
var incrBackupSkippedCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "backup_incremental_skip_total",
		Help: "Incremental backup skip counter.",
	})

// a counter metric for backup failure number
var backupFailureCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "backup_failure_total",
		Help: "Backup failure counter.",
	})

// a counter metric for incremental backup failure number
var incrBackupFailureCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "backup_incremental_failure_total",
		Help: "Incremental backup failure counter.",
	})

// a gauge metric for full backup duration
var backupDurationGauge = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "backup_duration_millis",
		Help: "Full backup duration in milliseconds.",
	})

// a gauge metric for incremental backup duration
var incrBackupDurationGauge = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "backup_incremental_duration_millis",
		Help: "Incremental backup duration in milliseconds.",
	})

func init() {
	prometheus.MustRegister(backupCounter)
	prometheus.MustRegister(incrBackupCounter)
	prometheus.MustRegister(backupSkippedCounter)
	prometheus.MustRegister(incrBackupSkippedCounter)
	prometheus.MustRegister(backupFailureCounter)
	prometheus.MustRegister(incrBackupFailureCounter)
	prometheus.MustRegister(backupDurationGauge)
	prometheus.MustRegister(incrBackupDurationGauge)
}
