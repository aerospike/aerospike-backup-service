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
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
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
	assert.Equal(t, model.JobStatusDone, jobStatus.Status)
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
	assert.Equal(t, model.JobStatusCanceled, jobStatus.Status)
}

func TestRestoreFailsWithClientError(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	storage := &model.LocalStorage{Path: "/backup/path"}
	request := model.NewRestoreRequest(*cluster, model.RestorePolicy{}, storage, nil, "/backup/path/data")

	clientErr := errors.New("connection error")
	env.mockClientManager.EXPECT().
		GetClient(gomock.Any(), cluster, gomock.Any()).
		Return(nil, clientErr)

	// Execute the restore
	jobID, err := env.restoreManager.Restore(t.Context(), request)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)

	require.NoError(t, err)
	assert.Equal(t, model.JobStatusFailed, jobStatus.Status)
	require.ErrorIs(t, jobStatus.Error, clientErr)
}

func TestRestoreFailsWithInvalidNamespace(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	cluster := &model.AerospikeCluster{}
	destinationNS := "test-ns"
	policy := model.RestorePolicy{
		Namespace: &model.RestoreNamespace{
			Destination: &destinationNS,
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
	assert.Equal(t, model.JobStatusFailed, jobStatus.Status)
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
	assert.Equal(t, model.JobStatusFailed, jobStatus.Status)
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
	assert.Equal(t, model.JobStatusFailed, jobStatus.Status)
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
	assert.Equal(t, model.JobStatusCanceled, jobStatus.Status)
	require.ErrorIs(t, jobStatus.Error, context.Canceled)
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
	restoreValidator  *MockRestoreValidator
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
	validator := NewMockRestoreValidator(ctrl)

	restoreManager := NewRestoreManager(
		mockRestore,
		mockClientManager,
		restoreJobsHolder,
		mockBackupReader,
		&collections.LockMap{},
		validator,
	)

	return &testRestoreEnv{
		ctrl:              ctrl,
		mockRestore:       mockRestore,
		mockClientManager: mockClientManager,
		mockBackupReader:  mockBackupReader,
		jobsHolder:        restoreJobsHolder,
		restoreValidator:  validator,
		restoreManager:    restoreManager,
		infoGetter:        infoGetter,
	}
}

// expectDefaultRestoreHandler creates a MockRestoreHandler with default EXPECTs:
// Wait returns nil, GetStats returns empty stats, GetMetrics returns empty metrics.
// Returns the handler for use in mockRestore.EXPECT().Run(..., handler, ...).
func (env *testRestoreEnv) expectDefaultRestoreHandler() *restoreexecutor.MockRestoreHandler {
	mockRestoreHandler := restoreexecutor.NewMockRestoreHandler(env.ctrl)
	mockRestoreHandler.EXPECT().Wait(gomock.Any()).Return(nil).AnyTimes()
	mockRestoreHandler.EXPECT().GetStats().Return(models.NewRestoreStats()).AnyTimes()
	mockRestoreHandler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()
	return mockRestoreHandler
}

// expectSuccessfulClientInteraction sets up the common GetClient/Close mock expectations.
// It returns the mocked client instance for potential further use in the test.
func (env *testRestoreEnv) expectSuccessfulClientInteraction(
	t *testing.T,
) aerospike.Client {
	t.Helper()

	client := aerospike.NewMockClient(env.ctrl)
	client.EXPECT().InfoClient().Return(env.infoGetter).AnyTimes()

	env.mockClientManager.EXPECT().
		GetClient(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(client, nil).
		AnyTimes()

	env.mockClientManager.EXPECT().
		Close(client).
		AnyTimes()

	return client
}

// TestRestoreByTime_UsesLastFullBackupAsBase verifies that restore-by-time uses the last full
// backup at or before request.Time as the base and fetches incrementals from that full's Created
// time up to request.Time (i.e. restore chain is based on the correct "first" backup).
// Mocks return data only when called with the expected filters.
func TestRestoreByTime_UsesLastFullBackupAsBase(t *testing.T) {
	env := setupTestRestoreEnv(t)
	defer env.ctrl.Finish()

	// All times in the past: full backup T1, request time T2, incremental created between T1 and T2.
	now := time.Now()
	fullCreated := now.Add(-2 * time.Hour)
	requestTime := now.Add(-1 * time.Hour)
	incrCreated := fullCreated.Add(30 * time.Minute)

	routine := &model.BackupRoutine{Name: "test-routine"}
	request := &model.RestoreTimestampRequest{
		DestinationCluster: model.AerospikeCluster{},
		Policy:             model.RestorePolicy{},
		Routine:            *routine,
		Time:               requestTime,
		DisableReordering:  true,
	}

	fullBackup := model.BackupDetails{
		BackupMetadata: model.BackupMetadata{
			Created:   fullCreated,
			Namespace: "ns1",
			FileCount: 1,
		},
		Key:     "full/path",
		Storage: &model.LocalStorage{},
	}
	incrBackup := model.BackupDetails{
		BackupMetadata: model.BackupMetadata{
			Created:   incrCreated,
			Namespace: "ns1",
			FileCount: 1,
		},
		Key:     "incr/path",
		Storage: &model.LocalStorage{},
	}

	env.restoreValidator.EXPECT().
		ValidateTimestamp(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	gomock.InOrder(
		env.mockBackupReader.EXPECT().
			GetBackups(gomock.Any(), fullBackupFilterMatcher{toTime: requestTime}).
			Return([]model.BackupDetails{fullBackup}, nil),
		env.mockBackupReader.EXPECT().
			GetBackups(gomock.Any(), incrementalFilterMatcher{fromTime: fullCreated, toTime: requestTime}).
			Return([]model.BackupDetails{incrBackup}, nil),
	)

	client := env.expectSuccessfulClientInteraction(t)
	// Restore runs executor once per backup in chain (full then incremental).
	gomock.InOrder(
		env.mockRestore.EXPECT().
			Run(gomock.Any(), client, restoreRequestPathMatcher{expectedPath: fullBackup.Key}).
			Return(env.expectDefaultRestoreHandler(), nil),
		env.mockRestore.EXPECT().
			Run(gomock.Any(), client, restoreRequestPathMatcher{expectedPath: incrBackup.Key}).
			Return(env.expectDefaultRestoreHandler(), nil),
	)

	jobID, err := env.restoreManager.RestoreByTime(t.Context(), request)
	require.NoError(t, err)
	jobStatus, err := waitForRestore(t, env.restoreManager, jobID)
	require.NoError(t, err)
	assert.Equal(t, model.JobStatusDone, jobStatus.Status)
}

func TestRestoreByTime_CompressionAndEncryptionHandling(t *testing.T) {
	tests := []struct {
		name              string
		policy            model.RestorePolicy
		backupEncryption  string
		backupCompression string
		shouldSucceed     bool
	}{
		{
			name:              "sets compression policy from backup",
			policy:            model.RestorePolicy{},
			backupCompression: "ZSTD",
			shouldSucceed:     true,
		},
		{
			name:             "fails when encrypted backup has no policy",
			policy:           model.RestorePolicy{},
			backupEncryption: "AES128",
			shouldSucceed:    false,
		},
		{
			name: "fails when encryption mode mismatches",
			policy: model.RestorePolicy{
				EncryptionPolicy: &model.EncryptionPolicy{Mode: "AES256"},
			},
			backupEncryption: "AES128",
			shouldSucceed:    false,
		},
		{
			name: "fails when encryption key is missing",
			policy: model.RestorePolicy{
				EncryptionPolicy: &model.EncryptionPolicy{Mode: "AES128"},
			},
			backupEncryption: "AES128",
			shouldSucceed:    false,
		},
		{
			name: "succeeds with valid encryption policy",
			policy: model.RestorePolicy{
				EncryptionPolicy: &model.EncryptionPolicy{
					Mode:   "AES128",
					KeyEnv: ptr.Of("AES_KEY"),
				},
			},
			backupEncryption: "AES128",
			shouldSucceed:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupTestRestoreEnv(t)
			defer env.ctrl.Finish()

			request := &model.RestoreTimestampRequest{
				DestinationCluster: model.AerospikeCluster{},
				Policy:             tt.policy,
				Routine:            model.BackupRoutine{Name: "test-routine"},
				Time:               time.Now(),
				DisableReordering:  true,
			}

			backup := model.BackupDetails{
				BackupMetadata: model.BackupMetadata{
					Created:     time.Now().Add(-1 * time.Hour),
					Namespace:   "ns1",
					Encryption:  tt.backupEncryption,
					Compression: tt.backupCompression,
					FileCount:   1,
				},
				Key:     "backup/path/test",
				Storage: &model.LocalStorage{},
			}

			env.mockBackupReader.EXPECT().
				GetBackups(gomock.Any(), gomock.Any()).
				Return([]model.BackupDetails{backup}, nil).
				Times(2)

			client := env.expectSuccessfulClientInteraction(t)
			if tt.shouldSucceed {
				env.restoreValidator.EXPECT().
					ValidateTimestamp(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).
					AnyTimes()
				// Restore runs executor once per backup in chain (full + incremental = 2).
				env.mockRestore.EXPECT().
					Run(gomock.Any(), client, gomock.Any()).
					Return(env.expectDefaultRestoreHandler(), nil).Times(2)
			} else {
				env.restoreValidator.EXPECT().
					ValidateTimestamp(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("validator failed")).
					AnyTimes()
			}

			jobID, err := env.restoreManager.RestoreByTime(t.Context(), request)
			require.NoError(t, err)

			jobStatus, err := waitForRestore(t, env.restoreManager, jobID)
			require.NoError(t, err)

			if tt.shouldSucceed {
				assert.Equal(t, model.JobStatusDone, jobStatus.Status)
			} else {
				assert.Equal(t, model.JobStatusFailed, jobStatus.Status)
				require.Error(t, jobStatus.Error)
			}
		})
	}
}
