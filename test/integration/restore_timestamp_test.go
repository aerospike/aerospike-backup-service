//go:build integration

package integration

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// TestRestoreByTimestampWorkflow runs full backup, incremental backup, empty incremental backup,
// then restores the namespace to a point in time via POST /v1/restore/timestamp.
// Subtests share env, backup storage, and Aerospike state.
func (s *Suite) TestRestoreByTimestampWorkflow() {
	e := s.setupEnv()

	var fullBackup, incrBackup dto.BackupDetails

	s.Run("full", func() {
		successCount := s.metricBackupSuccessEventCount(e, model.BackupTypeFull)

		s.triggerFullBackup(e)

		s.waitForMetricBackupSuccessEvent(e, model.BackupTypeFull, successCount)

		fullBackup = s.waitForFullBackup(e)

		s.assertBackupDetails(fullBackup, 0)
		s.assertBackupListed(e, fullBackup)

		s.assertMetricBackupSuccessEventCount(e, model.BackupTypeFull, successCount+1)
		s.assertMetricLastSuccessfulBackup(e, model.BackupTypeFull, fullBackup.Created)
	})

	s.Run("incremental", func() {
		s.seedRecords([]int{1})

		s.triggerIncrementalBackup(e)

		incrBackup = s.waitForIncrementalBackup(e, 1)

		s.assertBackupDetails(incrBackup, 1)
		s.assertIncrementalBackupListed(e, incrBackup)
		s.GreaterOrEqual(incrBackup.Created.Unix(), fullBackup.Created.Unix())

		s.assertMetricLastSuccessfulBackup(e, model.BackupTypeIncremental, incrBackup.Created)
	})

	s.Run("empty_incremental", func() {
		successCount := s.metricBackupSuccessEventCount(e, model.BackupTypeIncremental)

		s.triggerIncrementalBackup(e)

		s.waitForMetricBackupSuccessEvent(e, model.BackupTypeIncremental, successCount)

		s.assertIncrementalBackupCount(e, 1)

		s.assertMetricBackupSuccessEventCount(e, model.BackupTypeIncremental, successCount+1)
		s.assertMetricLastSuccessfulBackup(e, model.BackupTypeIncremental, incrBackup.Created)
	})

	s.Run("restore_by_timestamp", func() {
		successCount := s.metricRestoreSuccessEventCount(e)

		s.Require().NoError(s.client.Truncate(nil, namespace, "", nil))

		status := s.restoreByTimestamp(e, time.Now())

		s.Equal(dto.RestoreSuccess, status.Status)
		s.Equal(uint64(1), status.InsertedRecords)
		s.Empty(status.Error)

		s.assertRecordsRestored([]int{1})

		s.assertMetricRestoreSuccessEventCount(e, successCount+1)
	})
}
