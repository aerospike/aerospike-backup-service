package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type testMocks struct {
	ctrl           *gomock.Controller
	backupExecutor *backupexecutor.MockBackup
	backupHandler  *backupexecutor.MockBackupHandler
	backupWriter   *MockBackupWriter
}

func initMocks(t *testing.T) (testMocks, NamespaceBackupRunner) {
	t.Helper()
	ctrl := gomock.NewController(t)

	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockBackupHandler := backupexecutor.NewMockBackupHandler(ctrl)
	mockBackupWriter := NewMockBackupWriter(ctrl)

	executor := NewNamespaceBackupRunner(
		mockBackupExecutor,
		mockBackupWriter,
		NewPathService(nil),
	)

	return testMocks{
		ctrl:           ctrl,
		backupExecutor: mockBackupExecutor,
		backupHandler:  mockBackupHandler,
		backupWriter:   mockBackupWriter,
	}, executor
}

func TestRun_SuccessfulFullBackup(t *testing.T) {
	now := time.UnixMilli(123456789000)
	backupFolder := "test-routine/backup/123456789000/data/test-ns"

	mocks, runner := initMocks(t)
	routine := &model.BackupRoutine{Name: routineName, SourceCluster: &model.AerospikeCluster{}}
	timeBounds := model.TimeBounds{}
	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)
	backupStats.ReadRecords.Add(50)

	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), routine, timeBounds, testNamespace, backupFolder, gomock.Any(), gomock.Any()).
		Return(mocks.backupHandler, nil)

	mocks.backupHandler.EXPECT().
		Wait(gomock.Any()).
		Return(nil)

	mocks.backupHandler.EXPECT().
		GetStats().
		Return(backupStats)

	mocks.backupWriter.EXPECT().
		WriteBackupMetadata(gomock.Any(), routine, backupFolder, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ *model.BackupRoutine, _ string, metadata model.BackupMetadata) error {
			// Check that metadata contains expected data
			assert.Equal(t, testNamespace, metadata.Namespace)
			assert.Equal(t, now, metadata.Created)
			assert.Equal(t, time.Time{}, metadata.From) // full backup
			assert.Equal(t, uint64(50), metadata.RecordCount)
			return nil
		})

	spec := model.BackupRunSpec{Type: model.BackupTypeFull, StartTime: now, TimeBounds: timeBounds}
	run := model.NamespaceRun{Routine: routine, Namespace: testNamespace, Spec: spec}
	handler, startErr := runner.Run(t.Context(), run, nil, slog.Default())
	require.NoError(t, startErr)

	require.NotNil(t, handler)
	err := handler.Wait(t.Context())
	require.NoError(t, err)
}

func TestSuccessfulIncrementalBackup(t *testing.T) {
	now := time.UnixMilli(123456789000)
	fromTime := time.UnixMilli(100000000000)
	backupFolder := "test-routine/incremental/123456789000/data/test-ns"

	mocks, runner := initMocks(t)
	routine := &model.BackupRoutine{Name: routineName, SourceCluster: &model.AerospikeCluster{}}
	timeBounds := model.TimeBounds{
		FromTime: &fromTime,
	}
	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(50)
	backupStats.ReadRecords.Add(25)

	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), routine, timeBounds, testNamespace, backupFolder, gomock.Any(), gomock.Any()).
		Return(mocks.backupHandler, nil)

	mocks.backupHandler.EXPECT().
		Wait(gomock.Any()).
		Return(nil)

	mocks.backupHandler.EXPECT().
		GetStats().
		Return(backupStats)

	mocks.backupWriter.EXPECT().
		WriteBackupMetadata(gomock.Any(), routine, backupFolder, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ *model.BackupRoutine, _ string, metadata model.BackupMetadata) error {
			// Check that metadata contains expected data
			assert.Equal(t, testNamespace, metadata.Namespace)
			assert.Equal(t, now, metadata.Created)
			assert.Equal(t, fromTime, metadata.From)
			assert.Equal(t, uint64(25), metadata.RecordCount)
			return nil
		})

	spec := model.BackupRunSpec{Type: model.BackupTypeIncremental, StartTime: now, TimeBounds: timeBounds}
	run := model.NamespaceRun{Routine: routine, Namespace: testNamespace, Spec: spec}
	handler, startErr := runner.Run(t.Context(), run, nil, slog.Default())
	require.NoError(t, startErr)

	require.NotNil(t, handler)
	err := handler.Wait(t.Context())
	require.NoError(t, err)
}

func TestEmptyIncrementalBackup(t *testing.T) {
	now := time.UnixMilli(123456789000)
	fromTime := time.UnixMilli(100000000000)
	backupFolder := "test-routine/incremental/123456789000/data/test-ns"

	mocks, runner := initMocks(t)
	routine := &model.BackupRoutine{Name: routineName, SourceCluster: &model.AerospikeCluster{}}
	timeBounds := model.TimeBounds{
		FromTime: &fromTime,
	}

	backupStats := models.NewBackupStats() // empty backup stats

	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), routine, timeBounds, testNamespace, backupFolder, gomock.Any(), gomock.Any()).
		Return(mocks.backupHandler, nil)

	mocks.backupHandler.EXPECT().
		Wait(gomock.Any()).
		Return(nil)

	mocks.backupHandler.EXPECT().
		GetStats().
		Return(backupStats)

	// No WriteBackupMetadata call expected for empty incremental backup.

	spec := model.BackupRunSpec{Type: model.BackupTypeIncremental, StartTime: now, TimeBounds: timeBounds}
	run := model.NamespaceRun{Routine: routine, Namespace: testNamespace, Spec: spec}
	handler, startErr := runner.Run(t.Context(), run, nil, slog.Default())
	require.NoError(t, startErr)

	require.NotNil(t, handler)
	err := handler.Wait(t.Context())
	require.NoError(t, err)
}

func TestBackupExecutorError(t *testing.T) {
	now := time.UnixMilli(123456789000)
	backupFolder := "test-routine/backup/123456789000/data/test-ns"

	mocks, runner := initMocks(t)
	routine := &model.BackupRoutine{
		Name:          routineName,
		SourceCluster: &model.AerospikeCluster{},
		BackupPolicy: &model.BackupPolicy{
			RetryPolicy: &model.RetryPolicy{
				MaxRetries: optional.Of(0),
			},
		},
	}
	timeBounds := model.TimeBounds{}

	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), routine, timeBounds, testNamespace, backupFolder, gomock.Any(), gomock.Any()).
		Return(nil, errors.New("nsRunner error"))

	// backup fails to start => nothing is written via backupWriter
	spec := model.BackupRunSpec{Type: model.BackupTypeFull, StartTime: now, TimeBounds: timeBounds}
	run := model.NamespaceRun{Routine: routine, Namespace: testNamespace, Spec: spec}
	handler, err := runner.Run(t.Context(), run, nil, slog.Default())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to start backup")
	require.Nil(t, handler)
}

// A namespace that fails deletes nothing: its folder has no metadata, so it stays out of the
// catalog, and the timestamp folder above it holds the other namespaces of the run.
func TestBackupHandlerError(t *testing.T) {
	now := time.UnixMilli(123456789000)
	backupFolder := "test-routine/backup/123456789000/data/test-ns"

	mocks, runner := initMocks(t)
	routine := &model.BackupRoutine{
		Name:          routineName,
		SourceCluster: &model.AerospikeCluster{},
		BackupPolicy: &model.BackupPolicy{
			RetryPolicy: &model.RetryPolicy{
				MaxRetries: optional.Of(0),
			},
		},
	}
	timeBounds := model.TimeBounds{}

	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), routine, timeBounds, testNamespace, backupFolder, gomock.Any(), gomock.Any()).
		Return(mocks.backupHandler, nil)

	mocks.backupHandler.EXPECT().
		Wait(gomock.Any()).
		Return(errors.New("handler error"))

	spec := model.BackupRunSpec{Type: model.BackupTypeFull, StartTime: now, TimeBounds: timeBounds}
	run := model.NamespaceRun{Routine: routine, Namespace: testNamespace, Spec: spec}
	handler, startErr := runner.Run(t.Context(), run, nil, slog.Default())
	require.NoError(t, startErr)

	require.NotNil(t, handler)
	err := handler.Wait(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "backup failed")
}

func TestMetadataWriteError(t *testing.T) {
	now := time.UnixMilli(123456789000)
	backupFolder := "test-routine/backup/123456789000/data/test-ns"

	mocks, runner := initMocks(t)
	routine := &model.BackupRoutine{
		Name:          routineName,
		SourceCluster: &model.AerospikeCluster{},
		BackupPolicy: &model.BackupPolicy{
			RetryPolicy: &model.RetryPolicy{
				MaxRetries: optional.Of(0),
			},
		},
	}
	timeBounds := model.TimeBounds{}
	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	metadataError := errors.New("metadata write error")
	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), routine, timeBounds, testNamespace, backupFolder, gomock.Any(), gomock.Any()).
		Return(mocks.backupHandler, nil)

	mocks.backupHandler.EXPECT().
		Wait(gomock.Any()).
		Return(nil)

	mocks.backupHandler.EXPECT().
		GetStats().
		Return(backupStats)

	mocks.backupWriter.EXPECT(). // no metadata => the folder stays out of the catalog, nothing is deleted
					WriteBackupMetadata(gomock.Any(), routine, backupFolder, gomock.Any()).
					Return(metadataError)

	spec := model.BackupRunSpec{Type: model.BackupTypeFull, StartTime: now, TimeBounds: timeBounds}
	run := model.NamespaceRun{Routine: routine, Namespace: testNamespace, Spec: spec}
	handler, startErr := runner.Run(t.Context(), run, nil, slog.Default())
	require.NoError(t, startErr)

	require.NotNil(t, handler)
	err := handler.Wait(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to write backup metadata")
}

func TestRetryableBackupHandler_Cancel(t *testing.T) {
	now := time.UnixMilli(123456789000)
	backupFolder := "test-routine/backup/123456789000/data/test-ns"

	mocks, runner := initMocks(t)
	routine := &model.BackupRoutine{Name: routineName, SourceCluster: &model.AerospikeCluster{}}
	timeBounds := model.TimeBounds{}

	var wg sync.WaitGroup
	wg.Add(1)

	// Set up minimal expectations to create a handler
	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), routine, timeBounds, testNamespace, backupFolder, gomock.Any(), gomock.Any()).
		Return(mocks.backupHandler, nil)

	mocks.backupHandler.EXPECT().
		Wait(gomock.Any()).
		DoAndReturn(func(ctx context.Context) error {
			// Signal that Wait was called
			wg.Done()
			// This will hang until context is canceled
			<-ctx.Done()
			return ctx.Err()
		})

	handler, startErr := runner.Run(
		t.Context(),
		model.NamespaceRun{
			Routine:   routine,
			Namespace: testNamespace,
			Spec:      model.BackupRunSpec{Type: model.BackupTypeFull, StartTime: now, TimeBounds: timeBounds},
		},
		nil,
		slog.Default(),
	)
	require.NoError(t, startErr)

	// Wait for the handler.Wait to be called
	wg.Wait()

	// Cancel the handler
	handler.Cancel()

	// Assert - the Wait method should eventually return with context.Canceled
	err := handler.Wait(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "context canceled")
}

// A retry writes to the next attempt folder under the same timestamp, so the run keeps one
// timestamp and metadata keeps the run's start time. Once the namespace is backed up, the folders
// of the failed attempts are removed, and only after the metadata is written.
func TestRun_RetryWritesNextAttemptFolder(t *testing.T) {
	startTime := time.UnixMilli(123456789000)
	attempt1 := "test-routine/backup/123456789000/data/test-ns"
	attempt2 := "test-routine/backup/123456789000/data/test-ns.2"
	attempt3 := "test-routine/backup/123456789000/data/test-ns.3"

	mocks, runner := initMocks(t)
	routine := retryingRoutine(2)
	timeBounds := model.TimeBounds{}

	failed := failingBackupHandler(mocks.ctrl, 2)

	backupStats := models.NewBackupStats()
	mocks.backupHandler.EXPECT().Wait(gomock.Any()).Return(nil)
	mocks.backupHandler.EXPECT().GetStats().Return(backupStats).AnyTimes()

	gomock.InOrder(
		mocks.backupExecutor.EXPECT().
			Run(gomock.Any(), routine, timeBounds, testNamespace, attempt1, gomock.Any(), gomock.Any()).
			Return(failed, nil),
		mocks.backupExecutor.EXPECT().
			Run(gomock.Any(), routine, timeBounds, testNamespace, attempt2, gomock.Any(), gomock.Any()).
			Return(failed, nil),
		mocks.backupExecutor.EXPECT().
			Run(gomock.Any(), routine, timeBounds, testNamespace, attempt3, gomock.Any(), gomock.Any()).
			Return(mocks.backupHandler, nil),
		mocks.backupWriter.EXPECT().
			WriteBackupMetadata(gomock.Any(), routine, attempt3, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ *model.BackupRoutine, _ string, md model.BackupMetadata) error {
				assert.Equal(t, startTime, md.Created, "a retry keeps the run's start time")
				return nil
			}),
		mocks.backupWriter.EXPECT().Delete(gomock.Any(), routine, attempt1).Return(nil),
		mocks.backupWriter.EXPECT().Delete(gomock.Any(), routine, attempt2).Return(nil),
	)

	spec := model.BackupRunSpec{Type: model.BackupTypeFull, StartTime: startTime, TimeBounds: timeBounds}
	run := model.NamespaceRun{Routine: routine, Namespace: testNamespace, Spec: spec}
	handler, startErr := runner.Run(t.Context(), run, nil, slog.Default())
	require.NoError(t, startErr)

	require.NoError(t, handler.Wait(t.Context()))
}

// Removing a failed attempt is housekeeping on a namespace that is already backed up: storage may
// not allow deletes, so a failure is logged and the backup still succeeds.
func TestRun_FailedAttemptRemovalErrorDoesNotFailBackup(t *testing.T) {
	startTime := time.UnixMilli(123456789000)
	attempt1 := "test-routine/backup/123456789000/data/test-ns"
	attempt2 := "test-routine/backup/123456789000/data/test-ns.2"

	mocks, runner := initMocks(t)
	routine := retryingRoutine(1)
	timeBounds := model.TimeBounds{}

	failed := failingBackupHandler(mocks.ctrl, 1)
	mocks.backupHandler.EXPECT().Wait(gomock.Any()).Return(nil)
	mocks.backupHandler.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()

	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), routine, timeBounds, testNamespace, attempt1, gomock.Any(), gomock.Any()).
		Return(failed, nil)
	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), routine, timeBounds, testNamespace, attempt2, gomock.Any(), gomock.Any()).
		Return(mocks.backupHandler, nil)
	mocks.backupWriter.EXPECT().WriteBackupMetadata(gomock.Any(), routine, attempt2, gomock.Any()).Return(nil)
	mocks.backupWriter.EXPECT().Delete(gomock.Any(), routine, attempt1).Return(errors.New("access denied"))

	logger, logBuf := newTestLogger(t)
	spec := model.BackupRunSpec{Type: model.BackupTypeFull, StartTime: startTime, TimeBounds: timeBounds}
	run := model.NamespaceRun{Routine: routine, Namespace: testNamespace, Spec: spec}
	handler, startErr := runner.Run(t.Context(), run, nil, logger)
	require.NoError(t, startErr)

	require.NoError(t, handler.Wait(t.Context()))
	assert.Contains(t, logBuf.String(), "Failed to remove the folder of a failed backup attempt")
}

// An empty incremental backup writes no metadata, but it still replaces the attempts before it.
func TestRun_EmptyIncrementalRetryRemovesFailedAttempt(t *testing.T) {
	startTime := time.UnixMilli(123456789000)
	fromTime := time.UnixMilli(100000000000)
	attempt1 := "test-routine/incremental/123456789000/data/test-ns"
	attempt2 := "test-routine/incremental/123456789000/data/test-ns.2"

	mocks, runner := initMocks(t)
	routine := retryingRoutine(1)
	timeBounds := model.TimeBounds{FromTime: &fromTime}

	failed := failingBackupHandler(mocks.ctrl, 1)
	mocks.backupHandler.EXPECT().Wait(gomock.Any()).Return(nil)
	mocks.backupHandler.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()

	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), routine, timeBounds, testNamespace, attempt1, gomock.Any(), gomock.Any()).
		Return(failed, nil)
	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), routine, timeBounds, testNamespace, attempt2, gomock.Any(), gomock.Any()).
		Return(mocks.backupHandler, nil)
	mocks.backupWriter.EXPECT().Delete(gomock.Any(), routine, attempt1).Return(nil)

	spec := model.BackupRunSpec{Type: model.BackupTypeIncremental, StartTime: startTime, TimeBounds: timeBounds}
	run := model.NamespaceRun{Routine: routine, Namespace: testNamespace, Spec: spec}
	handler, startErr := runner.Run(t.Context(), run, nil, slog.Default())
	require.NoError(t, startErr)

	require.NoError(t, handler.Wait(t.Context()))
}

// A metadata write that fails is retried on its own; the failed attempts are removed once, after
// the write that succeeds.
func TestRun_MetadataRetryRemovesFailedAttemptsOnce(t *testing.T) {
	startTime := time.UnixMilli(123456789000)
	attempt1 := "test-routine/backup/123456789000/data/test-ns"
	attempt2 := "test-routine/backup/123456789000/data/test-ns.2"

	mocks, runner := initMocks(t)
	routine := retryingRoutine(2)
	timeBounds := model.TimeBounds{}

	failed := failingBackupHandler(mocks.ctrl, 1)
	mocks.backupHandler.EXPECT().Wait(gomock.Any()).Return(nil)
	mocks.backupHandler.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()

	gomock.InOrder(
		mocks.backupExecutor.EXPECT().
			Run(gomock.Any(), routine, timeBounds, testNamespace, attempt1, gomock.Any(), gomock.Any()).
			Return(failed, nil),
		mocks.backupExecutor.EXPECT().
			Run(gomock.Any(), routine, timeBounds, testNamespace, attempt2, gomock.Any(), gomock.Any()).
			Return(mocks.backupHandler, nil),
		mocks.backupWriter.EXPECT().
			WriteBackupMetadata(gomock.Any(), routine, attempt2, gomock.Any()).
			Return(errors.New("write failed")),
		mocks.backupWriter.EXPECT().
			WriteBackupMetadata(gomock.Any(), routine, attempt2, gomock.Any()).
			Return(nil),
		mocks.backupWriter.EXPECT().Delete(gomock.Any(), routine, attempt1).Return(nil),
	)

	spec := model.BackupRunSpec{Type: model.BackupTypeFull, StartTime: startTime, TimeBounds: timeBounds}
	run := model.NamespaceRun{Routine: routine, Namespace: testNamespace, Spec: spec}
	handler, startErr := runner.Run(t.Context(), run, nil, slog.Default())
	require.NoError(t, startErr)

	require.NoError(t, handler.Wait(t.Context()))
}

// retryingRoutine returns a routine that retries a failed backup maxRetries times without delay.
func retryingRoutine(maxRetries int) *model.BackupRoutine {
	return &model.BackupRoutine{
		Name:          routineName,
		SourceCluster: &model.AerospikeCluster{},
		BackupPolicy: &model.BackupPolicy{
			RetryPolicy: &model.RetryPolicy{
				MaxRetries:  optional.Of(maxRetries),
				BaseTimeout: optional.Of(time.Millisecond),
			},
		},
	}
}

// failingBackupHandler returns a backup handler whose pipeline fails the given number of times.
func failingBackupHandler(ctrl *gomock.Controller, times int) *backupexecutor.MockBackupHandler {
	failed := backupexecutor.NewMockBackupHandler(ctrl)
	failed.EXPECT().Wait(gomock.Any()).Return(errors.New("handler error")).Times(times)
	failed.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()

	return failed
}
