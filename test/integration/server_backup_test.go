//go:build integration

package integration

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
)

// TestServerRestoreByPath mirrors TestRestoreByPath with server-side backup: runs a full
// backup, restores it via POST /v1/restore/full into the destination cluster, and verifies the
// backup details, the restored data and the exposed restore metrics. The destination namespace
// holds records the backup does not: a strict restore leaves exactly the backed-up data.
func (s *ServerBackupSuite) TestServerRestoreByPath() {
	e := s.setupEnv(s.serverBackupConfig)
	s.seedRecordsIn(s.client, serverNamespace, []int{10, 20, 30})
	successCount := s.metricRestoreSuccessEventCount(e)

	s.putRecordIn(s.destinationClient, serverNamespace, 7, 70)
	s.putRecord(s.destinationClient, "strays", as.BinMap{ageBin: 71})

	triggered := time.Now()
	s.triggerFullBackup(e)
	fullBackup := s.waitForFinishedBackup(e, model.BackupTypeFull)
	s.assertServerBackupDetails(fullBackup, triggered)
	s.gap("record_count", gapRecordCount, func() { s.Equal(uint64(3), fullBackup.RecordCount) })

	status := s.restoreIntoDestination(e, fullBackup.Key)
	s.Equal(dto.RestoreSuccess, status.Status)
	s.gap("inserted_records", gapRestoreStats, func() { s.Equal(uint64(3), status.InsertedRecords) })
	s.Empty(status.Error)
	s.assertRecordsIn(s.destinationClient, serverNamespace, []int{10, 20, 30})
	s.Nil(s.recordBins(s.destinationClient, setName, 7), "a record the backup does not hold was kept")
	s.Nil(s.recordBins(s.destinationClient, "strays", 0), "a set the backup does not hold was kept")
	s.assertMetricRestoreSuccessEventCount(e, successCount+1)
}

// TestServerBackupRestoreWithIndexes mirrors TestBackupRestoreWithIndexes with server-side
// backup: backs up a secondary index and a set index and checks that restoring the backup into
// the destination cluster creates them there.
func (s *ServerBackupSuite) TestServerBackupRestoreWithIndexes() {
	const wantIndexes = 2 // age_sidx + set_sidx

	e := s.setupEnv(s.serverBackupConfig)

	task, err := s.client.CreateIndex(nil, serverNamespace, setName, "age_sidx", ageBin, as.NUMERIC)
	s.Require().NoError(err)
	s.Require().NoError(<-task.OnComplete())

	setTask, err := s.client.CreateSetIndex(nil, serverNamespace, setName, "set_sidx")
	s.Require().NoError(err)
	s.Require().NoError(<-setTask.OnComplete())

	s.T().Cleanup(func() {
		for _, client := range []*as.Client{s.client, s.destinationClient} {
			// Either may be missing on the destination when the test failed before restoring.
			_ = client.DropIndex(nil, serverNamespace, setName, "age_sidx")
			_ = client.DropIndex(nil, serverNamespace, setName, "set_sidx")
		}
	})

	// No records: the indexes are backed up and restored on their own.
	s.triggerFullBackup(e)
	fullBackup := s.waitForFinishedBackup(e, model.BackupTypeFull)
	s.gap("index_count", gapIndexCount, func() {
		s.Equal(uint64(wantIndexes), fullBackup.SecondaryIndexCount)
	})

	s.gap("restore", gapEmptyRestore, func() {
		restoreStatus := s.restoreIntoDestination(e, fullBackup.Key)

		s.Equal(uint64(wantIndexes), restoreStatus.IndexCount)
		indexes := s.destinationIndexes()
		s.Contains(indexes, "age_sidx")
		s.Contains(indexes, "set_sidx")
	})
}

// TestServerRestoreByTimestampWorkflow mirrors TestRestoreByTimestampWorkflow with server-side
// backup: runs full backup, incremental backup, empty incremental backup, then restores the
// namespace into the destination cluster to a point in time via POST /v1/restore/timestamp.
// Unlike the scan test, the full backup holds a record, which the incremental backup holds
// updated: the server cannot restore an empty backup, and the restore must end with the newest
// value. A point in time between the two backups restores the full backup alone, and one before
// the full backup restores nothing. Subtests share env, backup storage, and Aerospike state.
func (s *ServerBackupSuite) TestServerRestoreByTimestampWorkflow() {
	const (
		fullAge        = 10
		incrementalAge = 11
	)

	e := s.setupEnv(s.serverBackupConfig)

	var fullBackup, incrBackup dto.BackupDetails

	s.Run("full", func() {
		s.seedRecordsIn(s.client, serverNamespace, []int{fullAge})
		successCount := s.metricBackupSuccessEventCount(e, model.BackupTypeFull)
		s.triggerFullBackup(e)
		fullBackup = s.waitForFinishedBackup(e, model.BackupTypeFull)
		s.assertServerBackupListed(e, model.BackupTypeFull, fullBackup)
		s.assertMetricBackupSuccessEventCount(e, model.BackupTypeFull, successCount+1)
		s.assertMetricLastSuccessfulBackup(e, model.BackupTypeFull, fullBackup.Created)
		s.waitForLastBackup(e, model.BackupTypeFull, fullBackup.Created)
	})

	s.Run("incremental", func() {
		s.seedRecordsIn(s.client, serverNamespace, []int{incrementalAge})
		s.triggerIncrementalBackup(e)
		incrBackup = s.waitForFinishedBackup(e, model.BackupTypeIncremental)
		s.gap("record_count", gapRecordCount, func() { s.Equal(uint64(1), incrBackup.RecordCount) })
		s.assertServerBackupListed(e, model.BackupTypeIncremental, incrBackup)
		s.GreaterOrEqual(incrBackup.Created.Unix(), fullBackup.Created.Unix())
		s.assertMetricLastSuccessfulBackup(e, model.BackupTypeIncremental, incrBackup.Created)
		s.waitForLastBackup(e, model.BackupTypeIncremental, incrBackup.Created)
	})

	s.Run("empty_incremental", func() {
		successCount := s.metricBackupSuccessEventCount(e, model.BackupTypeIncremental)
		s.triggerIncrementalBackup(e)
		s.waitForFinishedBackup(e, model.BackupTypeIncremental)
		s.assertMetricBackupSuccessEventCount(e, model.BackupTypeIncremental, successCount+1)
		s.gap("not_listed", gapEmptyIncr, func() {
			s.assertIncrementalBackupCount(e, 1)
			s.assertMetricLastSuccessfulBackup(e, model.BackupTypeIncremental, incrBackup.Created)
		})
	})

	s.Run("restore_by_timestamp", func() {
		successCount := s.metricRestoreSuccessEventCount(e)
		status := s.restoreIntoDestinationByTimestamp(e, time.Now())
		s.Equal(dto.RestoreSuccess, status.Status)
		s.gap("inserted_records", gapRestoreStats, func() { s.Equal(uint64(1), status.InsertedRecords) })
		s.Empty(status.Error)
		s.assertRecordsIn(s.destinationClient, serverNamespace, []int{incrementalAge})
		s.assertMetricRestoreSuccessEventCount(e, successCount+1)
	})

	s.Run("restore_between_full_and_incremental", func() {
		between := fullBackup.Created.Add(incrBackup.Created.Sub(fullBackup.Created) / 2)
		s.Require().True(between.After(fullBackup.Created) && between.Before(incrBackup.Created))

		status := s.restoreIntoDestinationByTimestamp(e, between)
		s.Empty(status.Error)
		s.assertRecordsIn(s.destinationClient, serverNamespace, []int{fullAge})
	})

	s.Run("restore_before_full", func() {
		request := timestampRestoreRequest(fullBackup.Created.Add(-time.Second), destinationClusterName,
			dto.RestoreModeStrict)
		job := s.waitForRestoreEnd(e, s.startRestore(e, e.baseURL+"/v1/restore/timestamp", request),
			serverJobTimeout)

		s.Equal(dto.RestoreFailure, job.Status)
		s.Contains(job.Error, "no full backups found")
		// The failed restore left the destination as the last restore did.
		s.assertRecordsIn(s.destinationClient, serverNamespace, []int{fullAge})
	})
}

// TestServerRestoreByTimestampCumulativeWorkflow mirrors TestRestoreByTimestampCumulativeWorkflow
// with server-side backup: runs full backup, incremental backup, another incremental backup,
// then restores the namespace into the destination cluster to a point in time. Since the mode
// is cumulative, the second incremental should cover the first one.
func (s *ServerBackupSuite) TestServerRestoreByTimestampCumulativeWorkflow() {
	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.BackupPolicies[policyName].IncrMode = dto.IncrModeCumulative
	})

	var fullBackup, incrBackup1, incrBackup2 dto.BackupDetails

	s.Run("full", func() {
		s.triggerFullBackup(e)
		fullBackup = s.waitForFinishedBackup(e, model.BackupTypeFull)
		s.assertServerBackupListed(e, model.BackupTypeFull, fullBackup)
		s.waitForLastBackup(e, model.BackupTypeFull, fullBackup.Created)
	})

	s.Run("incremental_1", func() {
		s.seedRecordsIn(s.client, serverNamespace, []int{1})
		s.triggerIncrementalBackup(e)
		incrBackup1 = s.waitForFinishedBackup(e, model.BackupTypeIncremental)
		s.gap("record_count", gapRecordCount, func() { s.Equal(uint64(1), incrBackup1.RecordCount) })
		s.assertServerBackupListed(e, model.BackupTypeIncremental, incrBackup1)
		s.Greater(incrBackup1.Created.Unix(), fullBackup.Created.Unix())
		s.waitForLastBackup(e, model.BackupTypeIncremental, incrBackup1.Created)
	})

	s.Run("incremental_2", func() {
		s.seedRecordsIn(s.client, serverNamespace, []int{1, 2})
		s.triggerIncrementalBackup(e)
		// The second incremental should have 2 records (1 and 2) because it's cumulative.
		incrBackup2 = s.waitForFinishedBackup(e, model.BackupTypeIncremental)
		s.gap("record_count", gapRecordCount, func() { s.Equal(uint64(2), incrBackup2.RecordCount) })
		s.assertServerBackupListed(e, model.BackupTypeIncremental, incrBackup2)
		s.Greater(incrBackup2.Created.Unix(), incrBackup1.Created.Unix())
		s.True(incrBackup2.From.Equal(fullBackup.Created), "cumulative incremental from %s, want %s",
			incrBackup2.From, fullBackup.Created)
	})

	// The chain starts with the full backup, which holds no records.
	s.gap("restore_by_timestamp", gapEmptyRestore, func() {
		status := s.restoreIntoDestinationByTimestamp(e, time.Now())
		s.Equal(dto.RestoreSuccess, status.Status)
		// Total inserted should be 2 from the second incremental.
		// The first incremental should be skipped.
		s.Equal(uint64(2), status.InsertedRecords)
		s.Empty(status.Error)
		s.assertRecordsIn(s.destinationClient, serverNamespace, []int{1, 2})
	})
}
