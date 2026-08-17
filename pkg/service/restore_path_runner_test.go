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
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRestoreOK(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	policy := model.RestorePolicy{}
	storage := &model.LocalStorage{Path: "/backup/path"}
	request := model.NewRestoreRequest(*cluster, policy, storage, nil, "/backup/path/data")

	client := env.expectSuccessfulClientInteraction(t)

	mockRestoreHandler := restoreexecutor.NewMockRestoreHandler(env.ctrl)
	stats := models.NewRestoreStats()
	stats.ReadRecords.Add(10)
	mockRestoreHandler.EXPECT().GetStats().Return(stats).AnyTimes()
	mockRestoreHandler.EXPECT().Wait(gomock.Any()).Return(nil)
	mockRestoreHandler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	env.mockRestore.EXPECT().
		Run(gomock.Any(), client, request).
		Return(mockRestoreHandler, nil)

	detailsDetails := model.BackupDetails{
		BackupMetadata: model.BackupMetadata{Created: time.Now(), Namespace: "test-ns", FileCount: 1},
	}
	env.mockBackupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(
		[]model.BackupDetails{detailsDetails}, nil)

	env.restoreValidator.EXPECT().
		ValidatePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	// Execute the restore
	jobID, err := env.restoreManager.Restore(t.Context(), request)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.RestoreSuccess, jobStatus.Status)
	assert.Equal(t, uint64(10), jobStatus.Counters.GetReadRecords(), "Read records count mismatch")
	require.NoError(t, jobStatus.Error, "Expected no error in final job status")
}

func TestCancelRestoreOK(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	policy := model.RestorePolicy{}
	storage := &model.LocalStorage{}
	request := model.NewRestoreRequest(*cluster, policy, storage, nil, "/backup/path/data")

	client := env.expectSuccessfulClientInteraction(t)

	waitReturned := make(chan struct{}) // To signal Wait has returned
	mockRestoreHandler := restoreexecutor.NewMockRestoreHandler(env.ctrl)
	mockRestoreHandler.EXPECT().GetStats().Return(models.NewRestoreStats()).AnyTimes()

	// Wait should block until context is canceled, then return context.Canceled
	mockRestoreHandler.EXPECT().Wait(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		<-ctx.Done()        // Wait for cancellation signal
		close(waitReturned) // Signal return
		return ctx.Err()    // Return cancellation error
	})
	mockRestoreHandler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	detailsDetails := model.BackupDetails{
		BackupMetadata: model.BackupMetadata{Created: time.Now(), Namespace: "test-ns", FileCount: 1},
	}
	env.mockBackupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(
		[]model.BackupDetails{detailsDetails}, nil)

	// Expect Run to start the process
	env.mockRestore.EXPECT().Run(gomock.Any(), client, request).Return(mockRestoreHandler, nil)

	env.restoreValidator.EXPECT().
		ValidatePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	jobID, err := env.restoreManager.Restore(t.Context(), request)
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
	assert.Equal(t, model.RestoreCanceled, jobStatus.Status)
}

func TestRestoreFailsWithClientError(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	storage := &model.LocalStorage{Path: "/backup/path"}
	request := model.NewRestoreRequest(*cluster, model.RestorePolicy{}, storage, nil, "/backup/path/data")

	clientErr := errors.New("connection error")
	env.mockClientManager.EXPECT().
		GetClient(gomock.Any(), cluster, nil, gomock.Any()).
		Return(nil, clientErr)

	// Execute the restore
	jobID, err := env.restoreManager.Restore(t.Context(), request)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.RestoreFailure, jobStatus.Status)
	require.ErrorIs(t, jobStatus.Error, clientErr)
}

func TestRestoreFailsWithInvalidNamespace(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	destinationNS := "test-ns"
	policy := model.RestorePolicy{
		Namespace: &model.RestoreNamespace{
			Destination: destinationNS,
		},
	}
	storage := &model.LocalStorage{Path: "/backup/path"}
	request := model.NewRestoreRequest(*cluster, policy, storage, nil, "/backup/path/data")

	env.expectSuccessfulClientInteraction(t)
	env.mockBackupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(
		[]model.BackupDetails{
			{BackupMetadata: model.BackupMetadata{Created: time.Now(), Namespace: "source-ns", FileCount: 1}},
		},
		nil,
	)
	env.restoreValidator.EXPECT().ValidatePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("destination cluster does not have required namespace: test-ns"))

	// Execute the restore
	jobID, err := env.restoreManager.Restore(t.Context(), request)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.RestoreFailure, jobStatus.Status)
	assert.Contains(t, jobStatus.Error.Error(),
		"destination cluster does not have required namespace: test-ns")
}

func TestRestoreFailsWithInvalidBackupData(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	policy := model.RestorePolicy{}
	storage := &model.LocalStorage{Path: "/backup/path"}
	request := model.NewRestoreRequest(*cluster, policy, storage, nil, "/backup/path/data")

	env.expectSuccessfulClientInteraction(t)
	env.mockBackupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(
		[]model.BackupDetails{
			{
				BackupMetadata: model.BackupMetadata{
					Created:   time.Now(),
					Namespace: "test-ns",
					FileCount: 1,
				},
			},
		},
		nil,
	)
	env.restoreValidator.EXPECT().ValidatePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("backups from different times were found"))

	// Execute the restore
	jobID, err := env.restoreManager.Restore(t.Context(), request)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.RestoreFailure, jobStatus.Status)
	assert.Contains(t, jobStatus.Error.Error(), "backups from different times were found")
}

func TestRestoreFailsWithRestoreServiceError(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	policy := model.RestorePolicy{}
	storage := &model.LocalStorage{Path: "/backup/path"}
	request := model.NewRestoreRequest(*cluster, policy, storage, nil, "/backup/path/data")

	client := env.expectSuccessfulClientInteraction(t)

	env.restoreValidator.EXPECT().
		ValidatePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	restoreErr := errors.New("restore service error")
	env.mockRestore.EXPECT().
		Run(gomock.Any(), client, request).
		Return(nil, restoreErr)

	detailsDetails := model.BackupDetails{
		BackupMetadata: model.BackupMetadata{Created: time.Now(), Namespace: "test-ns", FileCount: 1},
	}
	env.mockBackupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(
		[]model.BackupDetails{detailsDetails}, nil)

	// Execute the restore
	jobID, err := env.restoreManager.Restore(t.Context(), request)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.RestoreFailure, jobStatus.Status)
	assert.Contains(t, jobStatus.Error.Error(), "failed to start restore operation")
}

func TestCancelRestore_RaceCondition(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	storage := &model.LocalStorage{}
	policy := model.RestorePolicy{}
	request := model.NewRestoreRequest(*cluster, policy, storage, nil, "/backup/path/data")

	client := env.expectSuccessfulClientInteraction(t)
	env.mockBackupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(
		[]model.BackupDetails{
			{
				BackupMetadata: model.BackupMetadata{
					Created:   time.Now(),
					Namespace: "test-ns",
					FileCount: 1,
				},
			},
		},
		nil,
	)

	env.restoreValidator.EXPECT().
		ValidatePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

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
	jobID, err := env.restoreManager.Restore(t.Context(), request)
	require.NoError(t, err)

	// 2. Wait for the signal that the restore goroutine has called Run().
	// Use a channel with a timeout to avoid hanging the test.
	done := make(chan struct{})
	go func() {
		runStarted.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success, Run was called
	case <-time.After(5 * time.Second):
		// If we timed out, it means Run was never called.
		// Check the job status to see if it failed early.
		status, _ := env.restoreManager.JobStatus(jobID)
		t.Fatalf("Timed out waiting for Run() to start. Job status: %v, Error: %v", status.Status, status.Error)
	}

	// 3. Cancel the job while the Run() method is still "executing".
	err = env.restoreManager.CancelRestore(jobID)
	require.NoError(t, err)

	// 4. Wait for the job to complete.
	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.RestoreCanceled, jobStatus.Status)
	require.ErrorIs(t, jobStatus.Error, context.Canceled)
}
