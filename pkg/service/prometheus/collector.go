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

	backupRunningGauge.Reset()
	for routineName, rs := range runningState {
		if rs.Full != nil {
			backupRunningGauge.WithLabelValues(routineName, string(model.BackupTypeFull)).Set(1)
		}
		if rs.Incremental != nil {
			backupRunningGauge.WithLabelValues(routineName, string(model.BackupTypeIncremental)).Set(1)
		}
	}

	backupProgressGauge.Reset()
	backupProgressDeprecated.Reset()

	for routineName, currentStat := range runningState {
		if currentStat.Full != nil {
			pct := float64(currentStat.Full.PercentageDone)
			backupProgressGauge.WithLabelValues(routineName, string(model.BackupTypeFull)).Set(pct / 100)
			backupProgressDeprecated.WithLabelValues(routineName, string(model.BackupTypeFull)).Set(pct)
		}

		if currentStat.Incremental != nil {
			pct := float64(currentStat.Incremental.PercentageDone)
			backupProgressGauge.WithLabelValues(routineName, string(model.BackupTypeIncremental)).Set(pct / 100)
			backupProgressDeprecated.WithLabelValues(routineName, string(model.BackupTypeIncremental)).Set(pct)
		}
	}
}

func (mc *MetricsCollector) collectRestoreMetrics() {
	counts := mc.restores.StatusCounts()
	running := float64(counts[model.RestoreRunning])
	restoreRunningGauge.WithLabelValues().Set(running)
	restoreRunningGaugeDeprecated.WithLabelValues().Set(running)
}
