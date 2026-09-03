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

	s.Require().NoError(s.client.Truncate(nil, namespace, "", nil))

	status := s.restoreByPath(e, defaultRestoreRequest(fullBackup.Key))

	s.Equal(dto.RestoreSuccess, status.Status)
	s.Equal(uint64(3), status.InsertedRecords)
	s.Empty(status.Error)

	s.assertRecordsRestored([]int{10, 20, 30})

	s.assertMetricRestoreSuccessEventCount(e, successCount+1)
}

// TestBackupRestoreWithIndexes runs a backup-and-restore flow with secondary and set indexes.
func (s *BackupSuite) TestBackupRestoreWithIndexes() {
	e := s.setupEnv()

	var expectedIndexCount = uint64(1)

	// Create a secondary index on the "age" bin
	task, err := s.client.CreateIndex(nil, namespace, setName, "age_sidx", "age", as.NUMERIC)
	s.Require().NoError(err)
	s.Require().NoError(<-task.OnComplete())

	// TODO: uncomment when https://aerospike.atlassian.net/browse/BKRS-334 fixed
	// Create a set index on namespace & set
	// setTask, err := s.client.CreateSetIndex(nil, namespace, setName, "set_sidx")
	// s.Require().NoError(err)
	// s.Require().NoError(<-setTask.OnComplete())
	// expectedIndexCount++

	s.triggerFullBackup(e)

	fullBackup := s.waitForFullBackup(e)
	// We expect 1 secondary index and 1 set index.

	s.Require().Equal(expectedIndexCount, fullBackup.SecondaryIndexCount)

	// Drop both indexes
	s.Require().NoError(s.client.DropIndex(nil, namespace, setName, "age_sidx"))
	s.Require().NoError(s.client.DropIndex(nil, namespace, setName, "set_sidx"))

	restoreStatus := s.restoreByPath(e, defaultRestoreRequest(fullBackup.Key))

	// We expect 2 indexes to be successfully restored (1 secondary index + 1 set index)
	s.Equal(expectedIndexCount, restoreStatus.IndexCount)
}
