//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// TestServerRestoreByPath mirrors TestRestoreByPath with server-side backup: runs a full
// backup, restores it via POST /v1/restore/full into the destination cluster, and verifies the
// backup details, the restored data and the exposed restore metrics.
func (s *ServerBackupSuite) TestServerRestoreByPath() {
	e := s.setupEnv(s.serverBackupConfig)
	s.seedRecordsIn(s.client, serverNamespace, []int{10, 20, 30})
	successCount := s.metricRestoreSuccessEventCount(e)

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
	s.assertMetricRestoreSuccessEventCount(e, successCount+1)
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

	// The destination holds the record as the incremental backup has it: the restore writes the
	// full backup's version over it.
	s.Run("restore_between_full_and_incremental", func() {
		between := fullBackup.Created.Add(incrBackup.Created.Sub(fullBackup.Created) / 2)
		s.Require().True(between.After(fullBackup.Created) && between.Before(incrBackup.Created))

		status := s.restoreIntoDestinationByTimestamp(e, between)
		s.Empty(status.Error)
		s.assertRecordsIn(s.destinationClient, serverNamespace, []int{fullAge})
	})

	s.Run("restore_before_full", func() {
		request := timestampRestoreRequest(fullBackup.Created.Add(-time.Second), destinationClusterName)
		job := s.waitForRestoreEnd(e, s.startRestore(e, e.baseURL+"/v1/restore/timestamp", request),
			serverJobTimeout)

		s.Equal(dto.RestoreFailure, job.Status)
		s.Contains(job.Error, "no full backups found")
		// The failed restore left the destination as the last restore did.
		s.assertRecordsIn(s.destinationClient, serverNamespace, []int{fullAge})
	})
}

// TestServerRestoreByTimestampIntoEmptyNamespace restores a full and an incremental backup by
// timestamp into a destination namespace that holds no records. An empty namespace is where
// restore by timestamp switches a scan restore to the unique policy, which applies a chain newest
// first; the server takes a chain only full backup first.
func (s *ServerBackupSuite) TestServerRestoreByTimestampIntoEmptyNamespace() {
	e := s.setupEnv(s.serverBackupConfig)

	s.seedRecordsIn(s.client, serverNamespace, []int{1})
	s.triggerFullBackup(e)
	fullBackup := s.waitForFinishedBackup(e, model.BackupTypeFull)
	s.waitForLastBackup(e, model.BackupTypeFull, fullBackup.Created)

	// Key 0 changes and key 1 is new: the incremental holds both, and must be applied last.
	s.seedRecordsIn(s.client, serverNamespace, []int{2, 3})
	s.triggerIncrementalBackup(e)
	incrBackup := s.waitForFinishedBackup(e, model.BackupTypeIncremental)
	s.waitForLastBackup(e, model.BackupTypeIncremental, incrBackup.Created)

	s.emptyNamespace(s.destinationClient, serverNamespace)

	status := s.restoreIntoDestinationByTimestamp(e, time.Now())

	s.Empty(status.Error)
	s.assertRecordsIn(s.destinationClient, serverNamespace, []int{2, 3})
}

// TestServerRestoreIntoSource backs up the source cluster, then updates one record, deletes
// another and adds a third, and restores the backup into the source cluster itself, which stays
// live. The updated and deleted records get their backed-up values back; the added record, which
// the backup does not hold, stays.
func (s *ServerBackupSuite) TestServerRestoreIntoSource() {
	e := s.setupEnv(s.serverBackupConfig)
	s.seedRecordsIn(s.client, serverNamespace, []int{10, 20, 30})

	s.triggerFullBackup(e)
	fullBackup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.putRecordIn(s.client, serverNamespace, 0, 99)
	s.deleteRecordIn(s.client, serverNamespace, 1)
	s.putRecordIn(s.client, serverNamespace, 3, 40)

	status := s.restoreByPathInto(e, fullBackup.Key, clusterName)

	s.Equal(dto.RestoreSuccess, status.Status)
	s.assertRecordsIn(s.client, serverNamespace, []int{10, 20, 30, 40})
}

// TestServerBackupCancel cancels a running full backup through POST /v1/backups/cancel and checks
// that the server aborts the job, nothing shows as running and nothing is listed.
func (s *ServerBackupSuite) TestServerBackupCancel() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig)
	s.triggerFullBackup(e)

	ok := s.eventually(serverJobTimeout, pollInterval, func() bool { return s.routineState(e).Full != nil })
	s.Require().True(ok, "the full backup never showed as running")

	status, body := s.do(e, http.MethodPost, fmt.Sprintf("%s/v1/backups/cancel/%s", e.baseURL, routineName), nil)
	s.Require().Equal(http.StatusAccepted, status, "cancel: %s", body)

	ok = s.eventually(serverJobTimeout, pollInterval, func() bool {
		return strings.Contains(s.info(s.client, "backup-status"), "state=ABORTED")
	})
	s.True(ok, "server job not aborted: backup-status %s", s.info(s.client, "backup-status"))

	ok = s.eventually(serverJobTimeout, pollInterval, func() bool { return s.routineState(e).Full == nil })
	s.True(ok, "the canceled backup still shows as running")
	s.Empty(s.getBackups(e, model.BackupTypeFull), "a canceled backup is listed")
}
