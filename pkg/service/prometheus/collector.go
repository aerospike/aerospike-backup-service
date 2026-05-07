package prometheus

import (
	"context"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// RunningBackupsRegistry defines the interface for managing running backups and their statuses.
type RunningBackupsRegistry interface {
	// GetRunningState returns statistics for all current backups.
	GetRunningState() map[string]model.RoutineState
}

// RestoreJobsHolder defines the interface retrieving restore jobs and their statuses.
type RestoreJobsHolder interface {
	// StatusCounts returns counts of restore jobs by status.
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

	// set backup running gauge.
	backupRunningGauge.Reset()
	for routineName, rs := range runningState {
		if rs.Full != nil {
			backupRunningGauge.WithLabelValues(routineName, string(model.BackupTypeFull)).Set(1)
		}
		if rs.Incremental != nil {
			backupRunningGauge.WithLabelValues(routineName, string(model.BackupTypeIncremental)).Set(1)
		}
	}

	// set backup progress gauge.
	backupProgressGauge.Reset()
	backupProgressDeprecated.Reset()

	for routineName, rs := range runningState {
		if rs.Full != nil {
			p := rs.Full.Progress
			backupProgressGauge.WithLabelValues(routineName, string(model.BackupTypeFull)).Set(p)
			backupProgressDeprecated.WithLabelValues(routineName, string(model.BackupTypeFull)).Set(p * 100)
		}

		if rs.Incremental != nil {
			p := rs.Incremental.Progress
			backupProgressGauge.WithLabelValues(routineName, string(model.BackupTypeIncremental)).Set(p)
			backupProgressDeprecated.WithLabelValues(routineName, string(model.BackupTypeIncremental)).Set(p * 100)
		}
	}
}

func (mc *MetricsCollector) collectRestoreMetrics() {
	counts := mc.restores.StatusCounts()
	running := float64(counts[model.RestoreRunning])
	restoreRunningGauge.WithLabelValues().Set(running)
	restoreRunningGaugeDeprecated.WithLabelValues().Set(running)
}
