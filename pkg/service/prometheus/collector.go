package prometheus

import (
	"context"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// CollectInterval is how often the service refreshes the collected gauges.
const CollectInterval = 1 * time.Second

type MetricsCollector struct {
	mu            sync.Mutex
	runningState  func() map[string]model.RoutineState
	restoreCounts func() map[model.RestoreState]int
}

// NewMetricsCollector returns a MetricsCollector.
func NewMetricsCollector(
	runningState func() map[string]model.RoutineState,
	restoreCounts func() map[model.RestoreState]int,
) *MetricsCollector {
	return &MetricsCollector{
		runningState:  runningState,
		restoreCounts: restoreCounts,
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
	runningState := mc.runningState()

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

	for routineName, rs := range runningState {
		if rs.Full != nil {
			p := rs.Full.Progress * 100 // convert ratio to percent
			backupProgressGauge.WithLabelValues(routineName, string(model.BackupTypeFull)).Set(p)
		}

		if rs.Incremental != nil {
			p := rs.Incremental.Progress * 100 // convert ratio to percent
			backupProgressGauge.WithLabelValues(routineName, string(model.BackupTypeIncremental)).Set(p)
		}
	}
}

func (mc *MetricsCollector) collectRestoreMetrics() {
	counts := mc.restoreCounts()
	running := float64(counts[model.RestoreRunning])
	restoreRunningGauge.WithLabelValues().Set(running)
}
