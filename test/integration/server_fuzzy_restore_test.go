//go:build integration

package integration

import (
	"net/http"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// TestServerFuzzyRestoreIntoSource backs up the source cluster, then updates one record, deletes
// another and adds a third, and restores the backup in fuzzy mode into the source cluster
// itself, which stays live. The updated and deleted records get their backed-up values back;
// the added record, which the backup does not hold, stays.
func (s *ServerBackupSuite) TestServerFuzzyRestoreIntoSource() {
	e := s.setupEnv(s.serverBackupConfig)
	s.seedRecordsIn(s.client, serverNamespace, []int{10, 20, 30})

	s.triggerFullBackup(e)
	fullBackup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.putRecordIn(s.client, serverNamespace, 0, 99)
	s.deleteRecordIn(s.client, serverNamespace, 1)
	s.putRecordIn(s.client, serverNamespace, 3, 40)

	status := s.restoreByPathInto(e, fullBackup.Key, clusterName,
		&dto.RestorePolicy{BaseRestorePolicy: dto.BaseRestorePolicy{Mode: dto.RestoreModeFuzzy}})

	s.Equal(dto.RestoreSuccess, status.Status)
	s.assertRecordsIn(s.client, serverNamespace, []int{10, 20, 30, 40})
}

// TestServerFuzzyRestoreByTimestamp backs up a full and an incremental backup of the source
// cluster, updates the record again, and restores the chain by timestamp in fuzzy mode into the
// source cluster: the record ends with its value from the incremental backup.
func (s *ServerBackupSuite) TestServerFuzzyRestoreByTimestamp() {
	const (
		fullAge        = 10
		incrementalAge = 11
	)

	e := s.setupEnv(s.serverBackupConfig)

	s.seedRecordsIn(s.client, serverNamespace, []int{fullAge})
	s.triggerFullBackup(e)
	fullBackup := s.waitForFinishedBackup(e, model.BackupTypeFull)
	s.waitForLastBackup(e, model.BackupTypeFull, fullBackup.Created)

	s.seedRecordsIn(s.client, serverNamespace, []int{incrementalAge})
	s.triggerIncrementalBackup(e)
	s.waitForFinishedBackup(e, model.BackupTypeIncremental)

	s.putRecordIn(s.client, serverNamespace, 0, 99)

	status := s.restoreByTimestampInto(e, time.Now(), clusterName, dto.RestoreModeFuzzy)

	s.Equal(dto.RestoreSuccess, status.Status)
	s.assertRecordsIn(s.client, serverNamespace, []int{incrementalAge})
}

// TestServerRestoreModeRequired sends a restore without a mode, which server mode requires, and
// expects it to be rejected before any job starts.
func (s *ServerBackupSuite) TestServerRestoreModeRequired() {
	e := s.setupEnv(s.serverBackupConfig)

	request := defaultRestoreRequest("261101T120000-aaaa")
	body, err := decoder.Marshal(request, decoder.JSON, false)
	s.Require().NoError(err)

	status, response := s.do(e, http.MethodPost, e.baseURL+"/v1/restore/full", body)
	s.Equal(http.StatusBadRequest, status)
	s.Contains(string(response), `"mode" required`)
}
