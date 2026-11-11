package serverbackup

import (
	"context"
	"errors"
	"fmt"
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

// restoreManagerEnv is a restore manager over mocks of the catalog and the destination cluster.
type restoreManagerEnv struct {
	manager       service.RestoreManager
	catalog       *service.MockBackupReader
	clientManager *aerospike.MockClientManager
	client        *aerospike.MockClient
	infoClient    *aerospike.MockInfoGetter
}

func newRestoreManagerEnv(t *testing.T) *restoreManagerEnv {
	t.Helper()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().InfoClient().Return(infoClient).AnyTimes()

	jobs := service.NewRestoreJobsHolder()
	jobs.Start(t.Context())

	clientManager := aerospike.NewMockClientManager(ctrl)
	catalog := service.NewMockBackupReader(ctrl)

	return &restoreManagerEnv{
		manager:       NewRestoreManager(clientManager, jobs, catalog, NewRestoreValidator()),
		catalog:       catalog,
		clientManager: clientManager,
		client:        client,
		infoClient:    infoClient,
	}
}

// expectClient expects the destination cluster to be connected to once, and released.
func (e *restoreManagerEnv) expectClient() {
	e.clientManager.EXPECT().GetClient(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(e.client, nil)
	e.clientManager.EXPECT().Close(e.client)
}

// expectStart expects one restore job to start in the namespace for the backups with
// wantBackupIDs. The server answers the start with the id and the error given.
func (e *restoreManagerEnv) expectStart(t *testing.T, namespace, wantBackupIDs, jobID string, err error) {
	t.Helper()

	e.infoClient.EXPECT().StartRestore(gomock.Any(), requestInNamespace(namespace)).DoAndReturn(
		func(_ context.Context, request *infomodels.RequestRestore) (string, error) {
			assert.Equal(t, wantBackupIDs, request.BackupIDs)

			return jobID, err
		})
}

// expectRestore expects one restore job to start in the namespace for the backups with
// wantBackupIDs, and to complete.
func (e *restoreManagerEnv) expectRestore(t *testing.T, namespace, wantBackupIDs string) {
	t.Helper()

	e.expectStart(t, namespace, wantBackupIDs, restoreJobID, nil)
	e.infoClient.EXPECT().GetRestoreStatus(gomock.Any(), namespace, restoreJobID).Return(
		&infomodels.ResponseRestoreState{State: infomodels.RestoreStateComplete}, nil)
}

// waitForJob waits for the restore job to end and returns its status.
func (e *restoreManagerEnv) waitForJob(t *testing.T, jobID model.RestoreJobID) *model.RestoreJobStatus {
	t.Helper()

	var status *model.RestoreJobStatus
	require.Eventually(t, func() bool {
		var err error
		status, err = e.manager.JobStatus(jobID)
		require.NoError(t, err)

		return status.Status != model.RestoreRunning
	}, 10*time.Second, 50*time.Millisecond)

	return status
}

// requestInNamespace matches the request to start a restore job in the namespace.
func requestInNamespace(namespace string) gomock.Matcher {
	return gomock.Cond(func(request *infomodels.RequestRestore) bool { return request.Namespace == namespace })
}

// pathFilter matches the filter that selects the backup with the key.
func pathFilter(key string) gomock.Matcher {
	return gomock.Cond(func(filter service.BackupFilter) bool {
		pathFilter, ok := filter.(*service.PathFilter)

		return ok && pathFilter.Path() == key
	})
}

// chainOf returns the chain of a full and an incremental backup of the namespace, and a request
// to restore the routine to a time after both.
func chainOf(namespace string) (full, incremental model.BackupDetails, request *model.RestoreTimestampRequest) {
	fullTime := time.Date(2026, 10, 20, 9, 1, 11, 0, time.UTC)

	full = chainBackup(fullID)
	full.Namespace = namespace
	full.Created = fullTime

	incremental = chainBackup(incr1ID)
	incremental.Namespace = namespace
	incremental.Created = fullTime.Add(10 * time.Minute)
	incremental.From = fullTime

	request = &model.RestoreTimestampRequest{
		RoutineName: "routine",
		Storage:     full.Storage,
		Time:        fullTime.Add(time.Hour),
	}

	return full, incremental, request
}

func TestRestoreManager_Restore(t *testing.T) {
	t.Parallel()
	env := newRestoreManagerEnv(t)

	backup := chainBackup(fullID)
	env.catalog.EXPECT().GetBackups(gomock.Any(), pathFilter(backup.Key)).Return([]model.BackupDetails{backup}, nil)
	env.expectClient()
	env.expectRestore(t, "bar", fullID)

	jobID := env.manager.Restore(&model.RestoreRequest{SourceStorage: backup.Storage, BackupDataPath: backup.Key})

	assert.Equal(t, model.RestoreSuccess, env.waitForJob(t, jobID).Status)
}

func TestRestoreManager_Restore_Fails(t *testing.T) {
	t.Parallel()

	backup := chainBackup(fullID)
	failure := errors.New("refused")

	tests := []struct {
		name    string
		expect  func(t *testing.T, env *restoreManagerEnv)
		wantErr string
	}{
		{
			name: "backup cannot be read",
			expect: func(_ *testing.T, env *restoreManagerEnv) {
				env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(nil, failure)
			},
			wantErr: fmt.Sprintf("failed to read server backup %s: refused", backup.Key),
		},
		{
			name: "cluster cannot be connected to",
			expect: func(_ *testing.T, env *restoreManagerEnv) {
				env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return([]model.BackupDetails{backup}, nil)
				env.clientManager.EXPECT().GetClient(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, failure)
			},
			wantErr: "refused",
		},
		{
			name: "server refuses the restore",
			expect: func(t *testing.T, env *restoreManagerEnv) {
				t.Helper()

				env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return([]model.BackupDetails{backup}, nil)
				env.expectClient()
				env.expectStart(t, "bar", fullID, "", failure)
			},
			wantErr: "failed to start server restore: refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := newRestoreManagerEnv(t)
			tt.expect(t, env)

			jobID := env.manager.Restore(&model.RestoreRequest{SourceStorage: backup.Storage, BackupDataPath: backup.Key})

			status := env.waitForJob(t, jobID)
			assert.Equal(t, model.RestoreFailure, status.Status)
			require.EqualError(t, status.Error, tt.wantErr)
		})
	}
}

// TestRestoreManager_RestoreByTime checks that the chain of a namespace, its full backup and the
// incremental backup after it, is restored by one server job.
func TestRestoreManager_RestoreByTime(t *testing.T) {
	t.Parallel()
	env := newRestoreManagerEnv(t)

	full, incremental, request := chainOf("bar")
	gomock.InOrder(
		env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return([]model.BackupDetails{full}, nil),
		env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return([]model.BackupDetails{incremental}, nil),
	)
	env.expectClient()
	env.expectRestore(t, "bar", fullID+","+incr1ID)

	jobID := env.manager.RestoreByTime(request)

	assert.Equal(t, model.RestoreSuccess, env.waitForJob(t, jobID).Status)
}

// TestRestoreManager_RestoreByTime_NamespaceFails checks that a namespace that fails to restore
// fails the job and is named in its error, and that the other namespaces are still restored.
func TestRestoreManager_RestoreByTime_NamespaceFails(t *testing.T) {
	t.Parallel()
	env := newRestoreManagerEnv(t)

	barFull, barIncremental, request := chainOf("bar")
	bazFull, bazIncremental, _ := chainOf("baz")
	gomock.InOrder(
		env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).
			Return([]model.BackupDetails{barFull, bazFull}, nil),
		env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).
			Return([]model.BackupDetails{barIncremental, bazIncremental}, nil),
	)
	env.expectClient()
	env.expectStart(t, "bar", fullID+","+incr1ID, "", errors.New("refused"))
	env.expectRestore(t, "baz", fullID+","+incr1ID)

	jobID := env.manager.RestoreByTime(request)

	status := env.waitForJob(t, jobID)
	assert.Equal(t, model.RestoreFailure, status.Status)
	require.EqualError(t, status.Error,
		"failed to restore routine routine, namespace bar by timestamp: failed to start server restore: refused")
}

// TestRestoreManager_CancelRestore checks that canceling a restore job aborts the server job
// that runs it.
func TestRestoreManager_CancelRestore(t *testing.T) {
	t.Parallel()
	env := newRestoreManagerEnv(t)

	backup := chainBackup(fullID)
	env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return([]model.BackupDetails{backup}, nil)
	env.expectClient()

	started := make(chan struct{})
	env.infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, *infomodels.RequestRestore) (string, error) {
			close(started)

			return restoreJobID, nil
		})
	env.infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", restoreJobID).Return(
		&infomodels.ResponseRestoreState{State: infomodels.RestoreStateActive}, nil).AnyTimes()
	env.infoClient.EXPECT().AbortRestore(gomock.Any(), "bar", restoreJobID).Return(nil)

	jobID := env.manager.Restore(&model.RestoreRequest{SourceStorage: backup.Storage, BackupDataPath: backup.Key})
	<-started
	require.NoError(t, env.manager.CancelRestore(jobID))

	assert.Equal(t, model.RestoreCanceled, env.waitForJob(t, jobID).Status)
}
