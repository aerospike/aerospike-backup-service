package prometheus

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func counterValue(t *testing.T, counter *prometheus.CounterVec, labels prometheus.Labels) float64 {
	t.Helper()
	return testutil.ToFloat64(counter.With(labels))
}

func TestObserveRestoreCompletion(t *testing.T) {
	tests := []struct {
		status  model.RestoreState
		outcome Outcome
	}{
		{model.RestoreSuccess, OutcomeSuccess},
		{model.RestoreFailure, OutcomeFailure},
		{model.RestoreCanceled, OutcomeCanceled},
	}

	for _, tt := range tests {
		t.Run(string(tt.outcome), func(t *testing.T) {
			before := testutil.ToFloat64(restoreCounter.WithLabelValues(string(tt.outcome)))
			ObserveRestoreCompletion(tt.status)
			after := testutil.ToFloat64(restoreCounter.WithLabelValues(string(tt.outcome)))
			assert.InDelta(t, before+1, after, 0.001)
		})
	}
}

func TestObserveBackupEvent(t *testing.T) {
	labels := prometheus.Labels{
		labelRoutine: "routine-a",
		labelType:    string(model.BackupTypeFull),
		labelOutcome: string(OutcomeSuccess),
	}
	before := counterValue(t, backupCounter, labels)

	ObserveBackupEvent("routine-a", model.BackupTypeFull, OutcomeSuccess, 2*time.Minute)

	after := counterValue(t, backupCounter, labels)
	assert.InDelta(t, before+1, after, 0.001)
}

func TestObserveBackupEvent_zeroDurationSkipsHistogram(t *testing.T) {
	require.NotPanics(t, func() {
		ObserveBackupEvent("routine-d", model.BackupTypeFull, OutcomeSuccess, 0)
	})
}

func TestSetLastBackupTimestamp(t *testing.T) {
	fullTime := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	incrTime := time.Date(2025, 1, 3, 4, 5, 6, 0, time.UTC)
	lastRun := model.NewBackupTime(fullTime, incrTime)

	SetLastBackupTimestamp("routine-b", lastRun)

	fullGauge := lastBackupTimestampGauge.WithLabelValues("routine-b", string(model.BackupTypeFull))
	incrGauge := lastBackupTimestampGauge.WithLabelValues("routine-b", string(model.BackupTypeIncremental))
	assert.InDelta(t, float64(fullTime.Unix()), testutil.ToFloat64(fullGauge), 0.001)
	assert.InDelta(t, float64(incrTime.Unix()), testutil.ToFloat64(incrGauge), 0.001)
}

func TestMetricsCollector_collectBackupMetrics(t *testing.T) {
	collector := NewMetricsCollector(func() map[string]model.RoutineState {
		return map[string]model.RoutineState{
			"routine-1": {
				Full:        &model.RunningJob{Progress: 0.42},
				Incremental: &model.RunningJob{Progress: 0.75},
			},
		}
	}, func() map[model.RestoreState]int { return nil })

	collector.collectBackupMetrics()

	fullRunning := backupRunningGauge.WithLabelValues("routine-1", string(model.BackupTypeFull))
	incrRunning := backupRunningGauge.WithLabelValues("routine-1", string(model.BackupTypeIncremental))
	assert.InDelta(t, float64(1), testutil.ToFloat64(fullRunning), 0.001)
	assert.InDelta(t, float64(1), testutil.ToFloat64(incrRunning), 0.001)

	fullProgress := backupProgressGauge.WithLabelValues("routine-1", string(model.BackupTypeFull))
	incrProgress := backupProgressGauge.WithLabelValues("routine-1", string(model.BackupTypeIncremental))
	assert.InDelta(t, 42, testutil.ToFloat64(fullProgress), 0.001)
	assert.InDelta(t, 75, testutil.ToFloat64(incrProgress), 0.001)
}

func TestMetricsCollector_collectRestoreMetrics(t *testing.T) {
	collector := NewMetricsCollector(
		func() map[string]model.RoutineState { return nil },
		func() map[model.RestoreState]int {
			return map[model.RestoreState]int{model.RestoreRunning: 3}
		},
	)

	collector.collectRestoreMetrics()

	assert.InDelta(t, float64(3), testutil.ToFloat64(restoreRunningGauge.WithLabelValues()), 0.001)
}

func TestMetricsCollector_collectMetrics_skipsWhenLocked(t *testing.T) {
	collector := NewMetricsCollector(
		func() map[string]model.RoutineState { return nil },
		func() map[model.RestoreState]int { return nil },
	)
	collector.mu.Lock()
	defer collector.mu.Unlock()

	require.NotPanics(t, func() {
		collector.collectMetrics()
	})
}
