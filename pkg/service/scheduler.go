package service

import (
	"context"
	"fmt"

	"github.com/aerospike/backup/internal/util"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/prometheus/client_golang/prometheus"
)

// backup service
var backupService shared.Backup = shared.NewBackup()

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
		Help: "Backup incremental runs counter.",
	})

func init() {
	prometheus.MustRegister(backupCounter)
	prometheus.MustRegister(incrBackupCounter)
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

func aerospikeClusterByName(name string, clusters []*model.AerospikeCluster) (*model.AerospikeCluster, error) {
	for _, cluster := range clusters {
		if *cluster.Name == name {
			return cluster, nil
		}
	}
	return nil, fmt.Errorf("cluster not found for %s", name)
}

func backupStorageByName(name string, storage []*model.BackupStorage) (*model.BackupStorage, error) {
	for _, st := range storage {
		if *st.Name == name {
			return st, nil
		}
	}
	return nil, fmt.Errorf("storage not found for %s", name)
}
