//go:build integration

package integration

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
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

// TestBackupRestoreWithIndexes runs a backup-and-restore flow with secondary and set indexes.
func (s *Suite) TestBackupRestoreWithIndexes() {
	e := s.setupEnv()

	// Create a secondary index on the "age" bin
	task, err := s.client.CreateIndex(nil, namespace, setName, "age_sidx", "age", as.NUMERIC)
	s.Require().NoError(err)
	s.Require().NoError(<-task.OnComplete())

	// Create a set index on namespace & set
	setTask, err := s.client.CreateSetIndex(nil, namespace, setName, "set_sidx")
	s.Require().NoError(err)
	s.Require().NoError(<-setTask.OnComplete())

	s.triggerFullBackup(e)

	fullBackup := s.waitForFullBackup(e)
	// We expect 1 secondary index and 1 set index.
	s.Require().Equal(uint64(2), fullBackup.SecondaryIndexCount)

	// Drop both indexes
	s.Require().NoError(s.client.DropIndex(nil, namespace, setName, "age_sidx"))
	s.Require().NoError(s.client.DropIndex(nil, namespace, setName, "set_sidx"))

	restoreStatus := s.restoreByPath(e, fullBackup.Key)

	// We expect 2 indexes to be successfully restored (1 secondary index + 1 set index)
	s.Equal(uint64(2), restoreStatus.IndexCount)
}
