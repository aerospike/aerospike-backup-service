package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// restoreManagerEnv is a restore manager over mocks of the catalog and the destination cluster.
type restoreManagerEnv struct {
	manager       service.RestoreManager
	catalog       *MockCatalog
	clientManager *aerospike.MockClientManager
	client        *aerospike.MockClient
	infoClient    *aerospike.MockInfoGetter
	// routineStorage holds the lock a restore shares with retention.
	routineStorage *collections.LockMap
}

func newRestoreManagerEnv(t *testing.T) *restoreManagerEnv {
	t.Helper()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().InfoClient().Return(infoClient).AnyTimes()

	jobs := service.NewRestoreJobsHolder()

	clientManager := aerospike.NewMockClientManager(ctrl)
	catalog := NewMockCatalog(ctrl)
	routineStorage := &collections.LockMap{}

	return &restoreManagerEnv{
		manager:        NewRestoreManager(clientManager, jobs, catalog, routineStorage, NewRestoreValidator()),
		catalog:        catalog,
		clientManager:  clientManager,
		client:         client,
		infoClient:     infoClient,
		routineStorage: routineStorage,
	}
}

// expectClient expects the destination cluster to be connected to once, and released.
func (e *restoreManagerEnv) expectClient() {
	e.clientManager.EXPECT().GetClient(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(e.client, nil)
	e.clientManager.EXPECT().Close(e.client)
}

// expectStart expects one restore job to start in the namespace for the backups with
// wantBackupIDs. The server answers the start with jobID and startErr.
func (e *restoreManagerEnv) expectStart(t *testing.T, namespace, wantBackupIDs, jobID string, startErr error) {
	t.Helper()

	e.infoClient.EXPECT().StartRestore(gomock.Any(), requestInNamespace(namespace)).DoAndReturn(
		func(_ context.Context, request *infomodels.RequestRestore) (string, error) {
			assert.Equal(t, wantBackupIDs, request.BackupIDs)

			return jobID, startErr
		})
}

// expectRestore expects one restore job to start in the namespace for the backups with
// wantBackupIDs, and to complete.
func (e *restoreManagerEnv) expectRestore(t *testing.T, namespace, wantBackupIDs string) {
	t.Helper()

	e.expectStart(t, namespace, wantBackupIDs, restoreJobID, nil)
	e.infoClient.EXPECT().GetRestoreStatus(gomock.Any(), namespace, restoreJobID).Return(completeStatus, nil)
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

// chainOf returns the chain of a full and an incremental backup of the namespace.
func chainOf(namespace string) (full, incremental model.BackupDetails) {
	fullTime := time.Date(2026, 10, 20, 9, 1, 11, 0, time.UTC)

	full = chainBackup(fullID)
	full.Namespace = namespace
	full.Created = fullTime

	incremental = chainBackup(incr1ID)
	incremental.Namespace = namespace
	incremental.Created = fullTime.Add(10 * time.Minute)
	incremental.From = fullTime

	return full, incremental
}

// restoreAfter returns a request to restore the routine to a time after the backup.
func restoreAfter(backup model.BackupDetails) *model.RestoreTimestampRequest {
	return &model.RestoreTimestampRequest{
		RoutineName: "routine",
		Storage:     backup.Storage,
		Time:        backup.Created.Add(time.Hour),
		Policy:      fuzzyPolicy,
	}
}

func TestRestoreManager_Restore(t *testing.T) {
	t.Parallel()
	env := newRestoreManagerEnv(t)

	backup := chainBackup(fullID)
	env.catalog.EXPECT().GetBackup(gomock.Any(), backup.Storage, backup.Key).Return(backup, nil)
	env.expectClient()
	env.expectRestore(t, "bar", fullID)

	jobID := env.manager.Restore(restoreRequestOf(backup))

	assert.Equal(t, model.RestoreSuccess, env.waitForJob(t, jobID).Status)
}

func TestRestoreManager_Restore_Fails(t *testing.T) {
	t.Parallel()

	backup := chainBackup(fullID)
	failure := errors.New("refused")

	tests := []struct {
		name    string
		key     string
		expect  func(t *testing.T, env *restoreManagerEnv)
		wantErr string
	}{
		{
			name: "backup cannot be read",
			key:  backup.Key,
			expect: func(_ *testing.T, env *restoreManagerEnv) {
				env.catalog.EXPECT().GetBackup(gomock.Any(), gomock.Any(), gomock.Any()).Return(model.BackupDetails{}, failure)
			},
			wantErr: fmt.Sprintf("failed to read server backup %s: refused", backup.Key),
		},
		{
			name:    "key names no routine",
			key:     "no-slash",
			expect:  func(*testing.T, *restoreManagerEnv) {},
			wantErr: `invalid server backup key "no-slash": want <routine>/<job id>`,
		},
		{
			name: "cluster cannot be connected to",
			key:  backup.Key,
			expect: func(_ *testing.T, env *restoreManagerEnv) {
				env.catalog.EXPECT().GetBackup(gomock.Any(), gomock.Any(), gomock.Any()).Return(backup, nil)
				env.clientManager.EXPECT().GetClient(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, failure)
			},
			wantErr: "refused",
		},
		{
			name: "server refuses the restore",
			key:  backup.Key,
			expect: func(t *testing.T, env *restoreManagerEnv) {
				t.Helper()

				env.catalog.EXPECT().GetBackup(gomock.Any(), gomock.Any(), gomock.Any()).Return(backup, nil)
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
			request := restoreRequestOf(backup)
			request.BackupDataPath = tt.key

			jobID := env.manager.Restore(request)

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

	full, incremental := chainOf("bar")
	gomock.InOrder(
		env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return([]model.BackupDetails{full}, nil),
		env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return([]model.BackupDetails{incremental}, nil),
	)
	env.expectClient()
	env.expectRestore(t, "bar", fullID+","+incr1ID)

	jobID := env.manager.RestoreByTime(restoreAfter(incremental))

	assert.Equal(t, model.RestoreSuccess, env.waitForJob(t, jobID).Status)
}

// TestRestoreManager_RestoreByTime_NamespaceFails checks that a namespace that fails to restore
// fails the job and is named in its error, and that the other namespaces are still restored.
func TestRestoreManager_RestoreByTime_NamespaceFails(t *testing.T) {
	t.Parallel()
	env := newRestoreManagerEnv(t)

	barFull, barIncremental := chainOf("bar")
	bazFull, bazIncremental := chainOf("baz")
	gomock.InOrder(
		env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).
			Return([]model.BackupDetails{barFull, bazFull}, nil),
		env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).
			Return([]model.BackupDetails{barIncremental, bazIncremental}, nil),
	)
	env.expectClient()
	env.expectStart(t, "bar", fullID+","+incr1ID, "", errors.New("refused"))
	env.expectRestore(t, "baz", fullID+","+incr1ID)

	jobID := env.manager.RestoreByTime(restoreAfter(barIncremental))

	status := env.waitForJob(t, jobID)
	assert.Equal(t, model.RestoreFailure, status.Status)
	require.EqualError(t, status.Error,
		"failed to restore routine routine, namespace bar by timestamp: failed to start server restore: refused")
}

// TestRestoreManager_HoldsRoutineStorage checks that a restore read-locks its routine from its
// first catalog read until the server job ends, and releases it afterwards. A read lock lets
// restores of one routine run side by side.
func TestRestoreManager_HoldsRoutineStorage(t *testing.T) {
	t.Parallel()

	full, incremental := chainOf("bar")

	tests := []struct {
		name  string
		start func(env *restoreManagerEnv, locked func()) model.RestoreJobID
	}{
		{
			name: "by path",
			start: func(env *restoreManagerEnv, locked func()) model.RestoreJobID {
				env.catalog.EXPECT().GetBackup(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(context.Context, model.Storage, string) (model.BackupDetails, error) {
						locked()

						return full, nil
					})

				return env.manager.Restore(restoreRequestOf(full))
			},
		},
		{
			name: "by time",
			start: func(env *restoreManagerEnv, locked func()) model.RestoreJobID {
				gomock.InOrder(
					env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).DoAndReturn(
						func(context.Context, service.BackupFilter) ([]model.BackupDetails, error) {
							locked()

							return []model.BackupDetails{full}, nil
						}),
					env.catalog.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return([]model.BackupDetails{incremental}, nil),
				)

				return env.manager.RestoreByTime(restoreAfter(incremental))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := newRestoreManagerEnv(t)
			retention := env.routineStorage.Get("routine")
			locked := func() {
				if retention.TryLock() {
					retention.Unlock() // else a restore that locks later blocks on it
					assert.Fail(t, "the routine was not locked during the restore")
				}
				if assert.True(t, retention.TryRLock(), "the restore write-locked the routine") {
					retention.RUnlock()
				}
			}

			env.expectClient()
			env.infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).Return(restoreJobID, nil)
			env.infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", restoreJobID).DoAndReturn(
				func(context.Context, string, string) (*infomodels.ResponseRestoreState, error) {
					locked()

					return completeStatus, nil
				})

			env.waitForJob(t, tt.start(env, locked))

			assert.True(t, retention.TryLock(), "the routine stayed locked after the restore")
		})
	}
}

// TestRestoreManager_WaitsForRetention checks that a restore that finds retention holding its
// routine reads the catalog only once retention lets go.
func TestRestoreManager_WaitsForRetention(t *testing.T) {
	t.Parallel()
	env := newRestoreManagerEnv(t)
	backup := chainBackup(fullID)

	var read atomic.Bool
	env.catalog.EXPECT().GetBackup(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, model.Storage, string) (model.BackupDetails, error) {
			read.Store(true)

			return backup, nil
		})
	env.expectClient()
	env.expectRestore(t, "bar", fullID)

	retention := env.routineStorage.Get("routine")
	retention.Lock()
	jobID := env.manager.Restore(restoreRequestOf(backup))

	assert.Never(t, read.Load, 200*time.Millisecond, 10*time.Millisecond, "the restore read during retention")
	retention.Unlock()

	assert.Equal(t, model.RestoreSuccess, env.waitForJob(t, jobID).Status)
}

// TestRestoreManager_CancelRestore checks that canceling a restore job aborts the server job
// that runs it.
func TestRestoreManager_CancelRestore(t *testing.T) {
	t.Parallel()
	env := newRestoreManagerEnv(t)

	backup := chainBackup(fullID)
	env.catalog.EXPECT().GetBackup(gomock.Any(), gomock.Any(), gomock.Any()).Return(backup, nil)
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

	jobID := env.manager.Restore(restoreRequestOf(backup))
	<-started
	require.NoError(t, env.manager.CancelRestore(jobID))

	assert.Equal(t, model.RestoreCanceled, env.waitForJob(t, jobID).Status)
}

// restoreRequestOf returns a request for a fuzzy restore of the backup.
func restoreRequestOf(backup model.BackupDetails) *model.RestoreRequest {
	return &model.RestoreRequest{SourceStorage: backup.Storage, BackupDataPath: backup.Key, Policy: fuzzyPolicy}
}
