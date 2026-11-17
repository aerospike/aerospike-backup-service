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
	mu sync.Mutex
	// routineStates returns the state of every configured routine by its name.
	routineStates func() map[string]model.RoutineState
	restoreCounts func() map[model.RestoreState]int
}

// NewMetricsCollector returns a MetricsCollector.
func NewMetricsCollector(
	routineStates func() map[string]model.RoutineState,
	restoreCounts func() map[model.RestoreState]int,
) *MetricsCollector {
	return &MetricsCollector{
		routineStates: routineStates,
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
	runningState := mc.routineStates()

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

	collectServerJobMetrics(runningState)
}

// collectServerJobMetrics sets the server job gauges of each routine and backup type. They are
// reset first, so the series of a routine or a namespace that is gone disappear.
func collectServerJobMetrics(states map[string]model.RoutineState) {
	serverBackupRecordsGauge.Reset()
	serverBackupBytesGauge.Reset()
	serverBackupRecordsReadFailedGauge.Reset()

	for routineName, state := range states {
		setServerJobGauges(routineName, model.BackupTypeFull, shownServerJobs(state.Full, state.LastFullJobs))
		setServerJobGauges(routineName, model.BackupTypeIncremental,
			shownServerJobs(state.Incremental, state.LastIncrementalJobs))
	}
}

// shownServerJobs returns the server jobs of the running backup, or of the last finished one
// when none runs.
func shownServerJobs(running *model.RunningJob, last []model.ServerBackupJob) []model.ServerBackupJob {
	if running != nil {
		return running.ServerJobs
	}

	return last
}

func setServerJobGauges(routineName string, backupType model.BackupType, jobs []model.ServerBackupJob) {
	for _, job := range jobs {
		labels := []string{routineName, string(backupType), job.Namespace}
		serverBackupRecordsGauge.WithLabelValues(labels...).Set(float64(job.RecordsBackedUp))
		serverBackupBytesGauge.WithLabelValues(labels...).Set(float64(job.BytesBackedUp))
		serverBackupRecordsReadFailedGauge.WithLabelValues(labels...).Set(float64(job.RecordsReadFailed))
	}
}

func (mc *MetricsCollector) collectRestoreMetrics() {
	counts := mc.restoreCounts()
	running := float64(counts[model.RestoreRunning])
	restoreRunningGauge.WithLabelValues().Set(running)
}
