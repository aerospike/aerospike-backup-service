//go:build integration

// Metrics assertions for integration tests: Prometheus counters and gauges.
//
// Keep assertMetric* and waitForMetric* helpers here rather than in *_test.go files so
// scenarios stay focused on setup and flow while metrics checks remain reusable.
package integration

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// metricBackupSuccessEventCount returns backup_events_total{outcome="success"} for the given type.
// Prometheus omits counters until the first event, so a missing series is treated as zero.
func (s *Suite) metricBackupSuccessEventCount(e *env, backupType model.BackupType) float64 {
	s.T().Helper()

	count, ok, err := e.backupSuccessEventCount(s.T().Context(), backupType)
	s.Require().NoError(err)
	if !ok {
		return 0
	}

	return count
}

// waitForMetricBackupSuccessEvent polls until backup_events_total{outcome="success"} increases.
func (s *Suite) waitForMetricBackupSuccessEvent(e *env, backupType model.BackupType, previousCount float64) {
	s.T().Helper()

	deadline := time.Now().Add(backupTimeout)

	for time.Now().Before(deadline) {
		if s.metricBackupSuccessEventCount(e, backupType) > previousCount {
			return
		}

		time.Sleep(pollInterval)
	}

	s.Failf("timed out waiting for %s backup success event", string(backupType))
}

func (s *Suite) assertMetricBackupSuccessEventCount(e *env, backupType model.BackupType, want float64) {
	s.T().Helper()

	count, ok, err := e.backupSuccessEventCount(s.T().Context(), backupType)
	s.Require().NoError(err)
	s.Require().True(ok, "backup success event counter not found for type %q", backupType)
	s.Equal(want, count)
}

// metricRestoreSuccessEventCount returns restore_events_total{outcome="success"}.
// Prometheus omits counters until the first event, so a missing series is treated as zero.
func (s *Suite) metricRestoreSuccessEventCount(e *env) float64 {
	s.T().Helper()

	count, ok, err := e.restoreSuccessEventCount(s.T().Context())
	s.Require().NoError(err)
	if !ok {
		return 0
	}

	return count
}

// assertMetricRestoreSuccessEventCount asserts restore_events_total{outcome="success"} equals want.
func (s *Suite) assertMetricRestoreSuccessEventCount(e *env, want float64) {
	s.T().Helper()

	count, ok, err := e.restoreSuccessEventCount(s.T().Context())
	s.Require().NoError(err)
	s.Require().True(ok, "restore success event counter not found")
	s.Equal(want, count)
}

// assertMetricLastSuccessfulBackup polls /metrics until last_successful_backup_timestamp matches want.
func (s *Suite) assertMetricLastSuccessfulBackup(e *env, backupType model.BackupType, want time.Time) {
	s.T().Helper()

	wantUnix := want.Unix()
	deadline := time.Now().Add(backupTimeout)

	for {
		value, ok, err := e.lastSuccessfulBackupTimestamp(s.T().Context(), backupType)
		s.Require().NoError(err)

		if ok && int64(value) == wantUnix {
			return
		}

		if time.Now().After(deadline) {
			if !ok {
				s.Failf("timed out waiting for last successful backup metric",
					"metric %q with routine=%q type=%q not found after %s",
					lastSuccessfulBackupTimestampMetric, routineName, backupType, backupTimeout)
			}

			s.Failf("timed out waiting for last successful backup metric",
				"metric %q with routine=%q type=%q: got %d, want %d after %s",
				lastSuccessfulBackupTimestampMetric, routineName, backupType, int64(value), wantUnix, backupTimeout)
		}

		time.Sleep(pollInterval)
	}
}
