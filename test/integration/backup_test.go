//go:build integration

package integration

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// TestBackup runs one full and one incremental backup against a single service instance.
// Subtests share env and Aerospike state.
func (s *Suite) TestBackup() {
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
}
