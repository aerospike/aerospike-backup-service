//go:build integration

package integration

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	as "github.com/aerospike/aerospike-client-go/v8"
)

// TestRestoreByPath runs a full backup, wipes the namespace, restores via POST /v1/restore/full,
// and verifies both the restored data and the exposed restore metrics.
func (s *BackupSuite) TestRestoreByPath() {
	e := s.setupEnv()

	s.seedRecords([]int{10, 20, 30})

	successCount := s.metricRestoreSuccessEventCount(e)

	s.triggerFullBackup(e)
	fullBackup := s.waitForFullBackup(e)
	s.assertBackupDetails(fullBackup, 3)

	s.truncateNamespace()

	status := s.restoreByPath(e, defaultRestoreRequest(fullBackup.Key))

	s.Equal(dto.RestoreSuccess, status.Status)
	s.Equal(uint64(3), status.InsertedRecords)
	s.Empty(status.Error)

	s.assertRecordsRestored([]int{10, 20, 30})

	s.assertMetricRestoreSuccessEventCount(e, successCount+1)
}

// TestBackupRestoreWithIndexes backs up a secondary index and a set index, drops
// both, and checks that restoring the backup recreates them.
func (s *BackupSuite) TestBackupRestoreWithIndexes() {
	const wantIndexes = 2 // age_sidx + set_sidx

	e := s.setupEnv()

	task, err := s.client.CreateIndex(nil, namespace, setName, "age_sidx", "age", as.NUMERIC)
	s.Require().NoError(err)
	s.Require().NoError(<-task.OnComplete())

	setTask, err := s.client.CreateSetIndex(nil, namespace, setName, "set_sidx")
	s.Require().NoError(err)
	s.Require().NoError(<-setTask.OnComplete())

	// No records: the indexes are backed up and restored on their own.
	s.triggerFullBackup(e)
	fullBackup := s.waitForFullBackup(e)
	s.Require().Equal(uint64(wantIndexes), fullBackup.SecondaryIndexCount)

	s.Require().NoError(s.client.DropIndex(nil, namespace, setName, "age_sidx"))
	s.Require().NoError(s.client.DropIndex(nil, namespace, setName, "set_sidx"))

	restoreStatus := s.restoreByPath(e, defaultRestoreRequest(fullBackup.Key))

	s.Equal(uint64(wantIndexes), restoreStatus.IndexCount)
}
