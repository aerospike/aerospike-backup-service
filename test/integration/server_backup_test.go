//go:build integration

package integration

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// TestServerBackupRestoreByPath runs a full server-side backup of the source cluster and
// restores it by job id into the destination cluster via POST /v1/restore/full.
func (s *ServerBackupSuite) TestServerBackupRestoreByPath() {
	ages := randomAges()
	s.seedRecordsIn(s.client, serverNamespace, ages)

	e := s.setupEnv(s.serverBackupConfig)
	s.triggerFullBackup(e)
	backup := s.waitForNewBackupWithin(e, model.BackupTypeFull, serverJobTimeout)
	s.Equal(serverNamespace, backup.Namespace)

	request := defaultRestoreRequest(backup.Key)
	request.DestinationClusterConfig.Name = destinationClusterName
	s.restoreWithin(e, e.baseURL+"/v1/restore/full", request, serverJobTimeout)

	s.assertRecordsIn(s.destinationClient, serverNamespace, ages)
}

// TestServerBackupRestoreByTimestamp runs two full server-side backups with different data and
// restores into the destination cluster at a time between them via POST /v1/restore/timestamp,
// which must pick the first. Incremental server backups are not covered: the server backup
// metadata does not tell them apart from full ones.
func (s *ServerBackupSuite) TestServerBackupRestoreByTimestamp() {
	e := s.setupEnv(s.serverBackupConfig)

	firstAges := randomAges()
	s.seedRecordsIn(s.client, serverNamespace, firstAges)
	s.triggerFullBackup(e)
	s.waitForNewBackupWithin(e, model.BackupTypeFull, serverJobTimeout)

	// Job ids carry whole seconds: keep the restore time and the second backup apart.
	time.Sleep(time.Second)
	restoreTime := time.Now()
	time.Sleep(time.Second)

	s.Require().NoError(s.client.Truncate(nil, serverNamespace, "", nil))
	s.seedRecordsIn(s.client, serverNamespace, randomAges())
	s.triggerFullBackup(e)
	s.waitForNewBackupWithin(e, model.BackupTypeFull, serverJobTimeout)

	s.restoreWithin(e, e.baseURL+"/v1/restore/timestamp", dto.RestoreTimestampRequest{
		DestinationClusterConfig: dto.DestinationClusterConfig{
			Name: destinationClusterName,
		},
		Policy:  &dto.TimestampRestorePolicy{},
		Time:    restoreTime.UnixMilli(),
		Routine: routineName,
	}, serverJobTimeout)

	s.assertRecordsIn(s.destinationClient, serverNamespace, firstAges)
}
