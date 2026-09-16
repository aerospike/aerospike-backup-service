//go:build integration

package integration

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
)

// TestRestoreByTimestampCumulativeWorkflow runs full backup, incremental backup,
// another incremental backup, then restores the namespace to a point in time.
// Since the mode is cumulative, the second incremental should cover the first one.
func (s *BackupSuite) TestRestoreByTimestampCumulativeWorkflow() {
	e := s.setupEnv(func(c *dto.Config) {
		p := c.BackupPolicies[policyName]
		p.IncrMode = dto.IncrModeCumulative
	})

	var fullBackup, incrBackup1, incrBackup2 dto.BackupDetails

	s.Run("full", func() {
		s.triggerFullBackup(e)

		fullBackup = s.waitForFullBackup(e)

		s.assertBackupDetails(fullBackup, 0)
		s.assertBackupListed(e, fullBackup)
	})

	s.Run("incremental_1", func() {
		s.seedRecords([]int{1})

		s.triggerIncrementalBackup(e)

		incrBackup1 = s.waitForIncrementalBackup(e, 1)

		s.assertBackupDetails(incrBackup1, 1)
		s.assertIncrementalBackupListed(e, incrBackup1)
		s.Greater(incrBackup1.Created.Unix(), fullBackup.Created.Unix())
	})

	s.Run("incremental_2", func() {
		s.seedRecords([]int{1, 2})

		s.triggerIncrementalBackup(e)

		// The second incremental should have 2 records (1 and 2) because it's cumulative.
		incrBackup2 = s.waitForIncrementalBackup(e, 2)

		s.assertBackupDetails(incrBackup2, 2)
		s.assertIncrementalBackupListed(e, incrBackup2)
		s.Greater(incrBackup2.Created.Unix(), incrBackup1.Created.Unix())
	})

	s.Run("restore_by_timestamp", func() {
		s.Require().NoError(s.client.Truncate(nil, namespace, "", nil))

		status := s.restoreByTimestamp(e, time.Now())

		s.Equal(dto.RestoreSuccess, status.Status)
		// Total inserted should be 2 from the second incremental.
		// The first incremental should be skipped.
		s.Equal(uint64(2), status.InsertedRecords)
		s.Empty(status.Error)

		s.assertRecordsRestored([]int{1, 2})
	})
}
