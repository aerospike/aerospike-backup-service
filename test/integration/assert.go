//go:build integration

package integration

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
)

// assertBackupToStorage points ABS at storage, runs a full backup of three freshly
// seeded records, and asserts all three are in it.
func (s *Suite) assertBackupToStorage(storage *dto.Storage) {
	s.seedRecords([]int{10, 20, 30})

	e := s.setupEnv(func(c *dto.Config) {
		c.Storage[storageName] = storage
	})
	s.triggerFullBackup(e)

	s.assertBackupDetails(s.waitForFullBackup(e), 3)
}

// assertBackupDetails checks that backup is a finished backup of the test namespace
// holding recordCount records.
func (s *Suite) assertBackupDetails(backup dto.BackupDetails, recordCount uint64) {
	s.T().Helper()

	s.Equal(namespace, backup.Namespace)
	s.Equal(recordCount, backup.RecordCount)
	s.False(backup.Created.IsZero())
	s.False(backup.Finished.IsZero())
	s.NotEmpty(backup.Key)
}

// assertBackupListed checks that GET /v1/backups/full/{routine} returns want unchanged.
func (s *Suite) assertBackupListed(e *env, want dto.BackupDetails) {
	s.assertListed(e, model.BackupTypeFull, want)
}

// assertIncrementalBackupListed checks that GET /v1/backups/incremental/{routine}
// returns want unchanged.
func (s *Suite) assertIncrementalBackupListed(e *env, want dto.BackupDetails) {
	s.assertListed(e, model.BackupTypeIncremental, want)
}

// assertIncrementalBackupCount checks how many incremental backups the routine lists.
func (s *Suite) assertIncrementalBackupCount(e *env, want int) {
	s.T().Helper()

	s.Len(s.getBackups(e, model.BackupTypeIncremental), want)
}

// assertRecordsRestored checks that the records seedRecords(ages) wrote are back.
func (s *Suite) assertRecordsRestored(ages []int) {
	s.T().Helper()

	for i, age := range ages {
		key, err := as.NewKey(namespace, setName, i)
		s.Require().NoError(err)

		record, err := s.client.Get(nil, key)
		s.Require().NoError(err)
		s.Require().NotNil(record)
		s.Equal(age, record.Bins["age"])
	}
}

// waitForMetricBackupSuccessEvent polls until backup_events_total{outcome="success"}
// for backupType rises above previousCount.
func (s *Suite) waitForMetricBackupSuccessEvent(e *env, backupType model.BackupType, previousCount int) {
	s.T().Helper()

	ok := s.eventually(backupTimeout, pollInterval, func() bool {
		return s.metricBackupSuccessEventCount(e, backupType) > previousCount
	})
	s.Require().True(ok, "timed out waiting for %s backup success event", backupType)
}

// assertMetricBackupSuccessEventCount checks backup_events_total{outcome="success"}
// for backupType, which must exist.
func (s *Suite) assertMetricBackupSuccessEventCount(e *env, backupType model.BackupType, want int) {
	s.T().Helper()

	count, ok := s.backupEventCount(e, backupType, outcomeSuccess)
	s.Require().True(ok, "backup success event counter not found for type %q", backupType)
	s.Equal(want, int(count))
}

// assertMetricRestoreSuccessEventCount checks restore_events_total{outcome="success"},
// which must exist.
func (s *Suite) assertMetricRestoreSuccessEventCount(e *env, want int) {
	s.T().Helper()

	count, ok := s.restoreSuccessEventCount(e)
	s.Require().True(ok, "restore success event counter not found")
	s.Equal(want, int(count))
}

// assertMetricLastSuccessfulBackup polls until last_successful_backup_timestamp for
// backupType equals want, to the second.
func (s *Suite) assertMetricLastSuccessfulBackup(e *env, backupType model.BackupType, want time.Time) {
	s.T().Helper()

	var (
		value float64
		found bool
	)

	ok := s.eventually(backupTimeout, pollInterval, func() bool {
		value, found = s.lastSuccessfulBackupTimestamp(e, backupType)

		return found && int64(value) == want.Unix()
	})
	s.Require().True(ok, "metric %q routine=%q type=%q: found=%t, got %d, want %d after %s",
		lastSuccessfulBackupTimestampMetric, routineName, backupType, found, int64(value), want.Unix(), backupTimeout)
}

func (s *Suite) assertListed(e *env, backupType model.BackupType, want dto.BackupDetails) {
	s.T().Helper()

	for _, backup := range s.getBackups(e, backupType) {
		if backup.Key != want.Key {
			continue
		}

		s.Equal(want.Created, backup.Created)
		s.assertBackupDetails(backup, want.RecordCount)

		return
	}

	s.Require().Failf("backup not listed", "%s backup %q not found", backupType, want.Key)
}
