package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/backup-go/mocks"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRestoreOK(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	policy := &model.RestorePolicy{}
	storage := &model.LocalStorage{Path: "/backup/path"}
	request := model.NewRestoreRequest(cluster, policy, storage, nil, "/backup/path/data")

	client := env.expectSuccessfulClientInteraction(t, cluster)

	mockRestoreHandler := restoreexecutor.NewMockRestoreHandler(env.ctrl)
	stats := models.NewRestoreStats()
	stats.ReadRecords.Add(10)
	mockRestoreHandler.EXPECT().GetStats().Return(stats).AnyTimes()
	mockRestoreHandler.EXPECT().Wait(gomock.Any()).Return(nil)
	mockRestoreHandler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	env.mockRestore.EXPECT().
		Run(gomock.Any(), client, request).
		Return(mockRestoreHandler, nil)

	detailsDetails := model.BackupDetails{BackupMetadata: model.BackupMetadata{Created: time.Now(), FileCount: 1}}
	env.mockBackupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(
		[]model.BackupDetails{detailsDetails}, nil)

	// Execute the restore
	jobID, err := env.restoreManager.Restore(ctx, request)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.JobStatusDone, jobStatus.Status)
	assert.Equal(t, uint64(10), jobStatus.Counters.GetReadRecords(), "Read records count mismatch")
	assert.Empty(t, jobStatus.Error, "Expected no error in final job status")
}

func TestCancelRestoreOK(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	policy := &model.RestorePolicy{}
	storage := &model.LocalStorage{}
	request := model.NewRestoreRequest(cluster, policy, storage, nil, "/backup/path/data")

	client := env.expectSuccessfulClientInteraction(t, cluster)

	waitReturned := make(chan struct{}) // To signal Wait has returned
	mockRestoreHandler := restoreexecutor.NewMockRestoreHandler(env.ctrl)
	mockRestoreHandler.EXPECT().GetStats().Return(models.NewRestoreStats()).AnyTimes()

	// Wait should block until context is cancelled, then return context.Canceled
	mockRestoreHandler.EXPECT().Wait(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		<-ctx.Done()        // Wait for cancellation signal
		close(waitReturned) // Signal return
		return ctx.Err()    // Return cancellation error
	})
	mockRestoreHandler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	detailsDetails := model.BackupDetails{BackupMetadata: model.BackupMetadata{Created: time.Now(), FileCount: 1}}
	env.mockBackupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(
		[]model.BackupDetails{detailsDetails}, nil)

	// Expect Run to start the process
	env.mockRestore.EXPECT().Run(gomock.Any(), client, request).Return(mockRestoreHandler, nil)

	jobID, err := env.restoreManager.Restore(ctx, request)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	// Give the restore goroutine a moment to start and register the handler
	assert.Eventually(t, func() bool {
		return env.jobsHolder.Size() > 0
	}, time.Second, 10*time.Millisecond)

	// Cancel the job
	err = env.restoreManager.CancelRestore(jobID)
	require.NoError(t, err, "Failed to cancel job")

	// Wait for the handler's Wait method to actually return due to cancellation
	select {
	case <-waitReturned:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Restore handler Wait() did not return after cancellation")
	}

	// Check final status reflects cancellation
	jobStatus, waitErr := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, waitErr)
	require.NotNil(t, jobStatus)
	assert.Equal(t, model.JobStatusCancelled, jobStatus.Status)
}

func TestRestoreFailsWithClientError(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	storage := &model.LocalStorage{Path: "/backup/path"}
	request := model.NewRestoreRequest(cluster, nil, storage, nil, "/backup/path/data")

	clientErr := errors.New("connection error")
	env.mockClientManager.EXPECT().
		GetClient(cluster).
		Return(nil, clientErr)

	// Execute the restore
	jobID, err := env.restoreManager.Restore(ctx, request)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.JobStatusFailed, jobStatus.Status)
	assert.ErrorIs(t, jobStatus.Error, clientErr)
}

func TestRestoreFailsWithInvalidNamespace(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	destinationNS := "test-ns"
	policy := &model.RestorePolicy{
		Namespace: &model.RestoreNamespace{
			Destination: &destinationNS,
		},
	}
	storage := &model.LocalStorage{Path: "/backup/path"}
	request := model.NewRestoreRequest(cluster, policy, storage, nil, "/backup/path/data")

	env.expectSuccessfulClientInteraction(t, cluster)

	env.infoGetter.EXPECT().GetNamespacesList().Return([]string{"other NS"}, nil)

	// Execute the restore
	jobID, err := env.restoreManager.Restore(ctx, request)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.JobStatusFailed, jobStatus.Status)
	assert.Contains(t, jobStatus.Error.Error(),
		"destination cluster does not have required namespace: test-ns")
}

func TestRestoreFailsWithInvalidBackupData(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	policy := &model.RestorePolicy{}
	storage := &model.LocalStorage{Path: "/backup/path"}
	request := model.NewRestoreRequest(cluster, policy, storage, nil, "/backup/path/data")

	env.expectSuccessfulClientInteraction(t, cluster)

	// BackupReader returns backups with different creation times, which is invalid
	backups := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.Now().Add(-time.Hour), FileCount: 1}},
		{BackupMetadata: model.BackupMetadata{Created: time.Now(), FileCount: 1}},
	}
	env.mockBackupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(backups, nil)

	// Execute the restore
	jobID, err := env.restoreManager.Restore(ctx, request)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.JobStatusFailed, jobStatus.Status)
	assert.Contains(t, jobStatus.Error.Error(), "backups from different times were found")
}

func TestRestoreFailsWithRestoreServiceError(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	policy := &model.RestorePolicy{}
	storage := &model.LocalStorage{Path: "/backup/path"}
	request := model.NewRestoreRequest(cluster, policy, storage, nil, "/backup/path/data")

	client := env.expectSuccessfulClientInteraction(t, cluster)

	restoreErr := errors.New("restore service error")
	env.mockRestore.EXPECT().
		Run(gomock.Any(), client, request).
		Return(nil, restoreErr)

	detailsDetails := model.BackupDetails{BackupMetadata: model.BackupMetadata{Created: time.Now(), FileCount: 1}}
	env.mockBackupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(
		[]model.BackupDetails{detailsDetails}, nil)

	// Execute the restore
	jobID, err := env.restoreManager.Restore(ctx, request)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.JobStatusFailed, jobStatus.Status)
	assert.Contains(t, jobStatus.Error.Error(), "failed to start restore operation")
}

func TestCancelRestore_RaceCondition(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	storage := &model.LocalStorage{}
	policy := &model.RestorePolicy{}
	request := model.NewRestoreRequest(cluster, policy, storage, nil, "/backup/path/data")

	client := env.expectSuccessfulClientInteraction(t, cluster)
	env.mockBackupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(
		[]model.BackupDetails{{BackupMetadata: model.BackupMetadata{Created: time.Now(), FileCount: 1}}}, nil)

	var runStarted sync.WaitGroup
	runStarted.Add(1)

	// Mock the Run() call to simulate a long-running startup.
	// It will block until its context is canceled.
	env.mockRestore.EXPECT().
		Run(gomock.Any(), client, request).
		DoAndReturn(func(
			ctx context.Context,
			_ aerospike.Client,
			_ *model.RestoreRequest,
		) (restoreexecutor.RestoreHandler, error) {
			// Signal that the Run method has started.
			runStarted.Done()
			// Now, wait until the context is canceled by the test.
			<-ctx.Done()
			// Return the cancellation error, as a real implementation would.
			return nil, ctx.Err()
		})

	// 1. Start the restore. This will run in a goroutine.
	jobID, err := env.restoreManager.Restore(ctx, request)
	require.NoError(t, err)

	// 2. Wait for the signal that the restore goroutine has called Run().
	runStarted.Wait()

	// 3. Cancel the job while the Run() method is still "executing".
	err = env.restoreManager.CancelRestore(jobID)
	require.NoError(t, err)

	// 4. Wait for the job to complete.
	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.JobStatusCancelled, jobStatus.Status)
	assert.ErrorIs(t, jobStatus.Error, context.Canceled)
}

func waitForRestore(
	t *testing.T,
	restoreManager RestoreManager,
	jobID model.RestoreJobID,
) (*model.RestoreJobStatus, error) {
	t.Helper()
	var (
		jobStatus *model.RestoreJobStatus
		err       error
	)

	assert.Eventually(t, func() bool {
		jobStatus, err = restoreManager.JobStatus(jobID)
		if err != nil {
			return false
		}
		return jobStatus.Status != model.JobStatusRunning
	}, 2*time.Second, 10*time.Millisecond, "Job should eventually stop running")

	return jobStatus, err
}

// testRestoreEnv holds the components needed for restore tests.
type testRestoreEnv struct {
	ctrl              *gomock.Controller
	mockRestore       *restoreexecutor.MockRestore
	mockClientManager *aerospike.MockClientManager
	infoGetter        *mocks.MockInfoGetter
	mockBackupReader  *MockBackupReader
	jobsHolder        *RestoreJobsHolder
	restoreManager    RestoreManager
}

// setupTestRestoreEnv initializes the test environment.
func setupTestRestoreEnv(t *testing.T) *testRestoreEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	infoGetter := mocks.NewMockInfoGetter(t)

	mockRestore := restoreexecutor.NewMockRestore(ctrl)
	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupReader := NewMockBackupReader(ctrl)
	restoreJobsHolder := NewRestoreJobsHolder()

	restoreManager := NewRestoreManager(
		mockRestore, mockClientManager, restoreJobsHolder, mockBackupReader, &collections.LockMap{})

	return &testRestoreEnv{
		ctrl:              ctrl,
		mockRestore:       mockRestore,
		mockClientManager: mockClientManager,
		mockBackupReader:  mockBackupReader,
		jobsHolder:        restoreJobsHolder,
		restoreManager:    restoreManager,
		infoGetter:        infoGetter,
	}
}

// expectSuccessfulClientInteraction sets up the common GetClient/Close mock expectations.
// It returns the mocked client instance for potential further use in the test.
func (env *testRestoreEnv) expectSuccessfulClientInteraction(
	t *testing.T,
	cluster *model.AerospikeCluster,
) aerospike.Client {
	t.Helper()

	client := aerospike.NewMockClient(env.ctrl)
	client.EXPECT().InfoClient().Return(env.infoGetter).AnyTimes()

	env.mockClientManager.EXPECT().
		GetClient(cluster).
		Return(client, nil).
		Times(1)

	env.mockClientManager.EXPECT().
		Close(client).
		Times(1)

	return client
}
