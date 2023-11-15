package service

import (
	"context"
	"github.com/aerospike/backup/internal/util"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/aerospike/backup/pkg/stdio"
	"github.com/prometheus/client_golang/prometheus"
)

// backup service
var backupService shared.Backup = shared.NewBackup()

// stdIO captures standard output
var stdIO *stdio.CgoStdio = &stdio.CgoStdio{}

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

func init() {
	prometheus.MustRegister(backupCounter)
	prometheus.MustRegister(incrBackupCounter)
	prometheus.MustRegister(backupSkippedCounter)
	prometheus.MustRegister(incrBackupSkippedCounter)
}

// ScheduleHandlers schedules the configured backup policies.
func ScheduleHandlers(ctx context.Context, handlers []BackupScheduler) {
	for _, handler := range handlers {
		handler.Schedule(ctx)
	}
}

// BuildBackupHandlers builds a list of BackupSchedulers according to
// the given configuration.
func BuildBackupHandlers(config *model.Config) []BackupScheduler {
	schedulers := make([]BackupScheduler, 0, len(config.BackupPolicy))
	for _, backupPolicy := range config.BackupPolicy {
		handler, err := NewBackupHandler(config, backupPolicy)
		util.Check(err)
		schedulers = append(schedulers, handler)
	}
	return schedulers
}

// ToBackend returns a list of underlying BackupBackends
// for the given list of BackupSchedulers.
func ToBackend(handlers []BackupScheduler) []BackupBackend {
	backends := make([]BackupBackend, 0, len(handlers))
	for _, scheduler := range handlers {
		backends = append(backends, scheduler.GetBackend())
	}
	return backends
}
