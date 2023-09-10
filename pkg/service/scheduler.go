package service

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/aerospike/backup/internal/util"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/prometheus/client_golang/prometheus"
)

// service logger
var logger *slog.Logger = slog.New(util.LogHandler)

// backup service
var backupService shared.Backup = shared.NewBackup()

// a counter metric for backup run number
var backupCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "backup_runs_total",
		Help: "Backup runs counter.",
	})

func init() {
	prometheus.MustRegister(backupCounter)
}

// ScheduleBackupJobs schedules the configured backups execution.
func ScheduleBackupJobs(ctx context.Context, config *model.Config) {
	for _, backupPolicy := range config.BackupPolicy {
		go scheduleBackup(ctx, config, backupPolicy)
	}
}

func scheduleBackup(ctx context.Context, config *model.Config, backupPolicy *model.BackupPolicy) {
	ticker := time.NewTicker(time.Duration(*backupPolicy.IntervalMillis) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cluster, err := aerospikeClusterByName(*backupPolicy.SourceCluster, config.AerospikeClusters)
			util.Check(err)
			storage, err := backupStorageByName(*backupPolicy.Storage, config.BackupStorage)
			util.Check(err)
			backupRunFunc := func() {
				backupService.BackupRun(backupPolicy, cluster, storage)
			}
			out := util.CaptureStdout(backupRunFunc)
			logger.Debug("Completed backup", "out", out)

			// increment backupCounter
			backupCounter.Inc()
		case <-ctx.Done():
			break
		}
	}
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
