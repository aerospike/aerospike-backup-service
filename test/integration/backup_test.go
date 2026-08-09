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

	var fullBackup dto.BackupDetails

	s.Run("full", func() {
		s.triggerFullBackup(e)

		fullBackup = s.waitForFullBackup(e)

		s.assertBackupDetails(fullBackup, 0)
		s.assertBackupMetadataOnDisk(e, fullBackup)
		s.assertLastSuccessfulBackupMetric(e, model.BackupTypeFull, fullBackup.Created)
	})

	s.Run("incremental", func() {
		s.seedRecords([]int{1})

		s.triggerIncrementalBackup(e)

		incrBackup := s.waitForIncrementalBackup(e, 1)

		s.assertBackupDetails(incrBackup, 1)
		s.assertBackupMetadataOnDisk(e, incrBackup)
		s.GreaterOrEqual(incrBackup.Created.Unix(), fullBackup.Created.Unix())
		s.assertLastSuccessfulBackupMetric(e, model.BackupTypeIncremental, incrBackup.Created)
	})
}
