package service

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/mocks" // Assuming this path for backup-go mocks
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

	env.mockRestore.EXPECT().
		Run(gomock.Any(), client, request).
		Return(mockRestoreHandler, nil)

	// Execute the restore
	jobID, err := env.restoreManager.Restore(request)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.JobStatusDone, jobStatus.Status)
	assert.Equal(t, uint64(10), jobStatus.ReadRecords, "Read records count mismatch")
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

	// Expect Run to start the process
	env.mockRestore.EXPECT().Run(gomock.Any(), client, request).Return(mockRestoreHandler, nil)

	jobID, err := env.restoreManager.Restore(request)
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
	mockNsValidator   *aerospike.MockNamespaceValidator
	mockBackupReader  *MockBackupReader // Assuming MockBackupReader exists
	jobsHolder        *RestoreJobsHolder
	restoreManager    RestoreManager
}

// setupTestRestoreEnv initializes the test environment.
func setupTestRestoreEnv(t *testing.T) *testRestoreEnv {
	t.Helper()
	ctrl := gomock.NewController(t)

	mockRestore := restoreexecutor.NewMockRestore(ctrl)
	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockNsValidator := aerospike.NewMockNamespaceValidator(ctrl)
	mockBackupReader := NewMockBackupReader(ctrl)
	restoreJobsHolder := NewRestoreJobsHolder()

	restoreManager := NewRestoreManager(
		mockRestore,
		mockClientManager,
		restoreJobsHolder,
		mockNsValidator,
		mockBackupReader,
	)

	return &testRestoreEnv{
		ctrl:              ctrl,
		mockRestore:       mockRestore,
		mockClientManager: mockClientManager,
		mockNsValidator:   mockNsValidator,
		mockBackupReader:  mockBackupReader,
		jobsHolder:        restoreJobsHolder,
		restoreManager:    restoreManager,
	}
}

// mockClient is a helper to create a backup client with a mock Aerospike client.
func mockClient(t *testing.T) *backup.Client {
	t.Helper()

	client, err := backup.NewClient(&mocks.MockAerospikeClient{})
	require.NoError(t, err, "Failed to create mock backup client")
	return client
}

// expectSuccessfulClientInteraction sets up the common GetClient/Close mock expectations.
// It returns the mocked client instance for potential further use in the test.
func (env *testRestoreEnv) expectSuccessfulClientInteraction(
	t *testing.T,
	cluster *model.AerospikeCluster,
) *backup.Client {
	t.Helper()
	client := mockClient(t)

	env.mockClientManager.EXPECT().
		GetClient(cluster).
		Return(client, nil).
		Times(1)

	env.mockClientManager.EXPECT().
		Close(client).
		Times(1)

	return client
}
