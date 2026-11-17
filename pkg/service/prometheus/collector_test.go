package prometheus

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

// TestCollectMetrics_ServerJobs checks that the server job gauges show the jobs of a routine's
// running backup, or of its last finished one when none runs, and that the series of a routine
// that is gone disappear.
func TestCollectMetrics_ServerJobs(t *testing.T) {
	states := map[string]model.RoutineState{
		"server": {
			Full: &model.RunningJob{ServerJobs: []model.ServerBackupJob{
				{Namespace: "bar", RecordsBackedUp: 10, BytesBackedUp: 100, RecordsReadFailed: 1},
			}},
			LastFullJobs: []model.ServerBackupJob{
				{Namespace: "bar", RecordsBackedUp: 99},
				{Namespace: "baz", RecordsBackedUp: 5},
			},
			LastIncrementalJobs: []model.ServerBackupJob{
				{Namespace: "bar", RecordsBackedUp: 7, BytesBackedUp: 70, RecordsReadFailed: 2},
			},
		},
		"scan": {Full: &model.RunningJob{}},
	}
	collector := NewMetricsCollector(
		func() map[string]model.RoutineState { return states },
		func() map[model.RestoreState]int { return nil },
	)
	gauges := []*prometheus.GaugeVec{
		serverBackupRecordsGauge, serverBackupBytesGauge, serverBackupRecordsReadFailedGauge,
	}

	collector.collectMetrics()

	// The running full backup hides the last one, whose baz job has no series. A scan backup has
	// no server jobs.
	for _, gauge := range gauges {
		assert.Equal(t, 2, testutil.CollectAndCount(gauge))
	}
	value := func(gauge *prometheus.GaugeVec, backupType model.BackupType) float64 {
		return testutil.ToFloat64(gauge.WithLabelValues("server", string(backupType), "bar"))
	}
	assert.InDelta(t, 10, value(serverBackupRecordsGauge, model.BackupTypeFull), 0)
	assert.InDelta(t, 100, value(serverBackupBytesGauge, model.BackupTypeFull), 0)
	assert.InDelta(t, 1, value(serverBackupRecordsReadFailedGauge, model.BackupTypeFull), 0)
	assert.InDelta(t, 7, value(serverBackupRecordsGauge, model.BackupTypeIncremental), 0)
	assert.InDelta(t, 70, value(serverBackupBytesGauge, model.BackupTypeIncremental), 0)
	assert.InDelta(t, 2, value(serverBackupRecordsReadFailedGauge, model.BackupTypeIncremental), 0)

	states = map[string]model.RoutineState{}
	collector.collectMetrics()

	for _, gauge := range gauges {
		assert.Zero(t, testutil.CollectAndCount(gauge))
	}
}
