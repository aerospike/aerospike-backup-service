//go:build integration

package integration

import (
	"math/rand/v2"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
)

// assertBackupRestoreViaStorage round-trips data through storage: the backup
// proves ABS can write with its credentials, the restore that it can read with them.
func (s *Suite) assertBackupRestoreViaStorage(storage *dto.Storage) {
	s.assertBackupRestore(s.client, func(c *dto.Config) {
		c.Storage[storageName] = storage
	})
}

// assertBackupRestore seeds a random number of records with random values through
// client, backs them up with the customized baseConfig, wipes the namespace,
// restores that backup through the same config, and checks every record is back.
//
// Random data is what makes the check specific to this run: a backup left over
// from an earlier test in the same storage would restore different values.
func (s *Suite) assertBackupRestore(client *as.Client, customize ...func(*dto.Config)) {
	ages := randomAges()

	s.Require().NoError(client.Truncate(nil, namespace, "", nil))
	s.seedRecordsWith(client, ages)

	e := s.setupEnv(customize...)
	s.triggerFullBackup(e)
	backup := s.waitForFullBackup(e)
	s.assertBackupDetails(backup, uint64(len(ages)))

	s.Require().NoError(client.Truncate(nil, namespace, "", nil))
	status := s.restoreByPath(e, defaultRestoreRequest(backup.Key))
	s.Equal(uint64(len(ages)), status.InsertedRecords)

	s.assertRecordsRestoredWith(client, ages)
}

// randomAges returns between 3 and 20 random values, for seedRecords.
func randomAges() []int {
	ages := make([]int, 3+rand.IntN(18)) //nolint:gosec // test data, not security
	for i := range ages {
		ages[i] = rand.IntN(1_000_000) //nolint:gosec // test data, not security
	}

	return ages
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
	s.assertRecordsRestoredWith(s.client, ages)
}

func (s *Suite) assertRecordsRestoredWith(client *as.Client, ages []int) {
	s.T().Helper()

	for i, age := range ages {
		key, err := as.NewKey(namespace, setName, i)
		s.Require().NoError(err)

		record, err := client.Get(nil, key)
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
