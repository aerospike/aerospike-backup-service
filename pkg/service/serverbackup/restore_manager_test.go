package serverbackup

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// restoreManagerEnv is a restore manager over mocks: a destination cluster that completes the
// one restore job it is asked to start, and a catalog.
type restoreManagerEnv struct {
	manager    service.RestoreManager
	catalog    *service.MockBackupReader
	infoClient *aerospike.MockInfoGetter
}

func newRestoreManagerEnv(t *testing.T) *restoreManagerEnv {
	t.Helper()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", restoreJobID).Return(
		&infomodels.ResponseRestoreState{State: infomodels.RestoreStateComplete}, nil)

	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().InfoClient().Return(infoClient).AnyTimes()

	clientManager := aerospike.NewMockClientManager(ctrl)
	clientManager.EXPECT().GetClient(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(client, nil)
	clientManager.EXPECT().Close(client)

	jobs := service.NewRestoreJobsHolder()
	jobs.Start(t.Context())

	catalog := service.NewMockBackupReader(ctrl)

	return &restoreManagerEnv{
		manager:    NewRestoreManager(clientManager, jobs, catalog, NopRestoreValidator()),
		catalog:    catalog,
		infoClient: infoClient,
	}
}

// expectStart expects one restore job to start for the backups with wantBackupIDs.
func (e *restoreManagerEnv) expectStart(t *testing.T, wantBackupIDs string) {
	t.Helper()

	e.infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *infomodels.RequestRestore) (string, error) {
			assert.Equal(t, wantBackupIDs, request.BackupIDs)
			assert.Equal(t, "bar", request.Namespace)

			return restoreJobID, nil
		})
}

// waitForJob waits for the restore job to end and returns its status.
func (e *restoreManagerEnv) waitForJob(t *testing.T, jobID model.RestoreJobID) model.RestoreState {
	t.Helper()

	var state model.RestoreState
	require.Eventually(t, func() bool {
		status, err := e.manager.JobStatus(jobID)
		require.NoError(t, err)
		state = status.Status

		return state != model.RestoreRunning
	}, 10*time.Second, 50*time.Millisecond)

	return state
}

func TestRestoreManager_Restore(t *testing.T) {
	t.Parallel()
	env := newRestoreManagerEnv(t)

	backup := chainBackup(fullID)
	env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return([]model.BackupDetails{backup}, nil)
	env.expectStart(t, fullID)

	jobID := env.manager.Restore(&model.RestoreRequest{SourceStorage: backup.Storage, BackupDataPath: backup.Key})

	assert.Equal(t, model.RestoreSuccess, env.waitForJob(t, jobID))
}

// TestRestoreManager_RestoreByTime checks that the chain of a namespace, its full backup and the
// incremental backup after it, is restored by one server job.
func TestRestoreManager_RestoreByTime(t *testing.T) {
	t.Parallel()
	env := newRestoreManagerEnv(t)

	fullTime := time.Date(2026, 10, 20, 9, 1, 11, 0, time.UTC)
	full := chainBackup(fullID)
	full.Created = fullTime
	incremental := chainBackup(incr1ID)
	incremental.Created = fullTime.Add(10 * time.Minute)
	incremental.From = fullTime

	gomock.InOrder(
		env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return([]model.BackupDetails{full}, nil),
		env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return([]model.BackupDetails{incremental}, nil),
	)
	env.expectStart(t, fullID+","+incr1ID)

	jobID := env.manager.RestoreByTime(&model.RestoreTimestampRequest{
		RoutineName: "routine",
		Storage:     full.Storage,
		Time:        fullTime.Add(time.Hour),
	})

	assert.Equal(t, model.RestoreSuccess, env.waitForJob(t, jobID))
}
