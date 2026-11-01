//go:build integration

package integration

import (
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// TestServerFuzzyRestoreIntoSource backs up the source cluster through ABS, then updates one
// record and deletes another, and restores the backup with restore-policy=fuzzy into the source
// cluster itself, which is live and holds data. It asks the server directly: ABS restores cold
// only, into an empty namespace.
func (s *ServerBackupSuite) TestServerFuzzyRestoreIntoSource() {
	e := s.setupEnv(s.serverBackupConfig)
	s.seedRecordsIn(s.client, serverNamespace, []int{10, 20, 30})

	s.triggerFullBackup(e)
	fullBackup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.putRecordIn(s.client, serverNamespace, 0, 99)
	deleteKey, keyErr := as.NewKey(serverNamespace, setName, 1)
	s.Require().NoError(keyErr)
	_, deleteErr := s.client.Delete(nil, deleteKey)
	s.Require().NoError(deleteErr)

	ctx := s.T().Context()
	infoClient, err := asinfo.NewClient(s.client.Cluster(), as.NewInfoPolicy(), nil, slog.Default())
	s.Require().NoError(err)

	jobID := time.Now().UTC().Format("060102T150405") + "-fzzy"
	s.Require().NoError(infoClient.StartRestore(ctx, &infomodels.RequestRestore{
		RequestCommon: infomodels.RequestCommon{
			Namespace: serverNamespace,
			Storage:   "aws-s3",
			Path:      s.backupPath(),
			Bucket:    s3Bucket,
			Region:    s3Region,
			Profile:   s3Profile,
			Endpoint:  s.s3Endpoint,
		},
		JobID:     jobID,
		BackupIDs: fullBackup.Key,
		Policy:    infomodels.RestorePolicyFuzzy,
	}))

	var status *infomodels.ResponseRestoreState
	done := s.eventually(serverJobTimeout, time.Second, func() bool {
		status, err = infoClient.GetRestoreStatus(ctx, serverNamespace, jobID)
		if err != nil {
			s.T().Logf("restore status: %v", err)
			return false
		}
		s.T().Logf("restore status: state=%s ns-state=%s", status.State, status.NsState)

		return status.State != infomodels.RestoreStateActive
	})
	s.Require().True(done, "fuzzy restore %s did not end", jobID)
	s.Require().Equal(infomodels.RestoreStateComplete, status.State, "error: %s", status.ErrorReason)

	s.T().Logf("updated record after restore: %v", s.recordBins(s.client, setName, 0))
	s.assertRecordsIn(s.client, serverNamespace, []int{10, 20, 30})
}
