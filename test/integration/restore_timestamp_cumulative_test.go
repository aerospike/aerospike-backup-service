//go:build integration

package integration

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// TestRestoreByTimestampCumulativeWorkflow runs full backup, incremental backup,
// another incremental backup, then restores the namespace to a point in time.
// Since the mode is cumulative, the second incremental should cover the first one.
func (s *BackupSuite) TestRestoreByTimestampCumulativeWorkflow() {
	e := s.setupEnv(func(c *dto.Config) {
		r := s.testRoutine(c)
		r.IncrMode = string(model.IncrModeCumulative)
	})

	var fullBackup, incrBackup1, incrBackup2 dto.BackupDetails

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

	s.Run("incremental_1", func() {
		s.seedRecords([]int{1})

		s.triggerIncrementalBackup(e)

		incrBackup1 = s.waitForIncrementalBackup(e, 1)

		s.assertBackupDetails(incrBackup1, 1)
		s.assertIncrementalBackupListed(e, incrBackup1)
		s.GreaterOrEqual(incrBackup1.Created.Unix(), fullBackup.Created.Unix())

		s.assertMetricLastSuccessfulBackup(e, model.BackupTypeIncremental, incrBackup1.Created)
	})

	s.Run("incremental_2", func() {
		// Use a different key by passing two elements, or write a custom insert.
		// seedRecords uses the index as the key.
		// So []int{0, 2} will insert key 0 with age 0, and key 1 with age 2.
		// Wait, key 0 was already inserted with age 1.
		// If we do []int{1, 2}, key 0 gets age 1, key 1 gets age 2.
		s.seedRecords([]int{1, 2})

		s.triggerIncrementalBackup(e)

		// The second incremental should have 2 records (1 and 2) because it's cumulative.
		incrBackup2 = s.waitForIncrementalBackup(e, 2)

		s.assertBackupDetails(incrBackup2, 2)
		s.assertIncrementalBackupListed(e, incrBackup2)
		s.GreaterOrEqual(incrBackup2.Created.Unix(), incrBackup1.Created.Unix())

		s.assertMetricLastSuccessfulBackup(e, model.BackupTypeIncremental, incrBackup2.Created)
	})

	s.Run("restore_by_timestamp", func() {
		successCount := s.metricRestoreSuccessEventCount(e)

		s.Require().NoError(s.client.Truncate(nil, namespace, "", nil))

		status := s.restoreByTimestamp(e, time.Now())

		s.Equal(dto.RestoreSuccess, status.Status)
		// Total inserted should be 2 from the second incremental.
		// The first incremental should be skipped.
		s.Equal(uint64(2), status.InsertedRecords)
		s.Empty(status.Error)

		s.assertRecordsRestored([]int{1, 2})

		s.assertMetricRestoreSuccessEventCount(e, successCount+1)
	})
}
