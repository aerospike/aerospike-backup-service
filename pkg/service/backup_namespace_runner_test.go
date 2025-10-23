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
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type testMocks struct {
	ctrl           *gomock.Controller
	backupExecutor *backupexecutor.MockBackup
	backupHandler  *backupexecutor.MockBackupHandler
	backendService *MockBackupReaderWriter
}

func initMocks(t *testing.T) (testMocks, *BackupNamespaceRunner) {
	t.Helper()
	ctrl := gomock.NewController(t)

	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockBackupHandler := backupexecutor.NewMockBackupHandler(ctrl)
	mockBackendService := NewMockBackupReaderWriter(ctrl)

	runner := NewBackupNamespaceRunner(
		routineName,
		mockBackupExecutor,
		&simpleExecutor{},
		mockBackendService,
		slog.Default(),
		NewPathService(nil),
	)

	return testMocks{
		ctrl:           ctrl,
		backupExecutor: mockBackupExecutor,
		backupHandler:  mockBackupHandler,
		backendService: mockBackendService,
	}, runner
}

func TestRun_SuccessfulFullBackup(t *testing.T) {
	now := time.UnixMilli(123456789000)
	backupFolder := "test-routine/backup/123456789000/data/test-ns"

	mocks, runner := initMocks(t)
	defer mocks.ctrl.Finish()

	client := &backup.Client{}
	backupRoutine := &model.BackupRoutine{}
	timeBounds := model.TimeBounds{}
	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)
	backupStats.ReadRecords.Add(50)

	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), client, backupRoutine, timeBounds, testNamespace, backupFolder).
		Return(mocks.backupHandler, nil)

	mocks.backupHandler.EXPECT().
		Wait(gomock.Any()).
		Return(nil)

	mocks.backupHandler.EXPECT().
		GetStats().
		Return(backupStats)

	mocks.backendService.EXPECT().
		WriteBackupMetadata(gomock.Any(), routineName, backupFolder, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ string, metadata model.BackupMetadata) error {
			// Check that metadata contains expected data
			assert.Equal(t, testNamespace, metadata.Namespace)
			assert.Equal(t, now, metadata.Created)
			assert.Equal(t, time.Time{}, metadata.From) // full backup
			assert.Equal(t, uint64(50), metadata.RecordCount)
			return nil
		})

	handler := runner.Run(ctx, client, backupRoutine, jobTypeFull, testNamespace, now, timeBounds)

	require.NotNil(t, handler)
	err := handler.Wait(ctx)
	require.NoError(t, err)
}

func TestSuccessfulIncrementalBackup(t *testing.T) {
	now := time.UnixMilli(123456789000)
	fromTime := time.UnixMilli(100000000000)
	backupFolder := "test-routine/incremental/123456789000/data/test-ns"

	mocks, runner := initMocks(t)
	defer mocks.ctrl.Finish()

	client := &backup.Client{}
	backupRoutine := &model.BackupRoutine{}
	timeBounds := model.TimeBounds{
		FromTime: &fromTime,
	}
	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(50)
	backupStats.ReadRecords.Add(25)

	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), client, backupRoutine, timeBounds, testNamespace, backupFolder).
		Return(mocks.backupHandler, nil)

	mocks.backupHandler.EXPECT().
		Wait(gomock.Any()).
		Return(nil)

	mocks.backupHandler.EXPECT().
		GetStats().
		Return(backupStats)

	mocks.backendService.EXPECT().
		WriteBackupMetadata(gomock.Any(), routineName, backupFolder, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ string, metadata model.BackupMetadata) error {
			// Check that metadata contains expected data
			assert.Equal(t, testNamespace, metadata.Namespace)
			assert.Equal(t, now, metadata.Created)
			assert.Equal(t, fromTime, metadata.From)
			assert.Equal(t, uint64(25), metadata.RecordCount)
			return nil
		})

	handler := runner.Run(ctx, client, backupRoutine, jobTypeIncremental, testNamespace, now, timeBounds)

	require.NotNil(t, handler)
	err := handler.Wait(ctx)
	require.NoError(t, err)
}

func TestEmptyIncrementalBackup(t *testing.T) {
	now := time.UnixMilli(123456789000)
	fromTime := time.UnixMilli(100000000000)
	backupFolder := "test-routine/incremental/123456789000/data/test-ns"

	mocks, runner := initMocks(t)
	defer mocks.ctrl.Finish()

	client := &backup.Client{}
	backupRoutine := &model.BackupRoutine{}
	timeBounds := model.TimeBounds{
		FromTime: &fromTime,
	}

	backupStats := models.NewBackupStats() // empty backup stats

	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), client, backupRoutine, timeBounds, testNamespace, backupFolder).
		Return(mocks.backupHandler, nil)

	mocks.backupHandler.EXPECT().
		Wait(gomock.Any()).
		Return(nil)

	mocks.backupHandler.EXPECT().
		GetStats().
		Return(backupStats)

	// No WriteBackupMetadata call expected for empty incremental backup.

	handler := runner.Run(ctx, client, backupRoutine, jobTypeIncremental, testNamespace, now, timeBounds)

	require.NotNil(t, handler)
	err := handler.Wait(ctx)
	require.NoError(t, err)
}

func TestBackupExecutorError(t *testing.T) {
	now := time.UnixMilli(123456789000)
	backupFolder := "test-routine/backup/123456789000/data/test-ns"

	mocks, runner := initMocks(t)
	defer mocks.ctrl.Finish()

	client := &backup.Client{}
	backupRoutine := &model.BackupRoutine{}
	timeBounds := model.TimeBounds{}

	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), client, backupRoutine, timeBounds, testNamespace, backupFolder).
		Return(nil, errors.New("executor error"))

	// backup fails to start => nothing is written via backendService
	handler := runner.Run(ctx, client, backupRoutine, jobTypeFull, testNamespace, now, timeBounds)

	require.NotNil(t, handler)
	err := handler.Wait(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to start backup")
}

func TestBackupHandlerError(t *testing.T) {
	now := time.UnixMilli(123456789000)
	backupFolder := "test-routine/backup/123456789000/data/test-ns"
	timestampPath := "test-routine/backup/123456789000"

	mocks, runner := initMocks(t)
	defer mocks.ctrl.Finish()

	client := &backup.Client{}
	backupRoutine := &model.BackupRoutine{}
	timeBounds := model.TimeBounds{}

	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), client, backupRoutine, timeBounds, testNamespace, backupFolder).
		Return(mocks.backupHandler, nil)

	mocks.backupHandler.EXPECT().
		Wait(gomock.Any()).
		Return(errors.New("handler error"))

	mocks.backendService.EXPECT(). // Important: delete all backup files, including configuration
					Delete(gomock.Any(), routineName, timestampPath).
					Return(nil)

	handler := runner.Run(ctx, client, backupRoutine, jobTypeFull, testNamespace, now, timeBounds)

	require.NotNil(t, handler)
	err := handler.Wait(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "backup failed")
}

func TestMetadataWriteError(t *testing.T) {
	now := time.UnixMilli(123456789000)
	backupFolder := "test-routine/backup/123456789000/data/test-ns"
	timestampPath := "test-routine/backup/123456789000"

	mocks, runner := initMocks(t)
	defer mocks.ctrl.Finish()

	client := &backup.Client{}
	backupRoutine := &model.BackupRoutine{}
	timeBounds := model.TimeBounds{}
	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	metadataError := errors.New("metadata write error")
	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), client, backupRoutine, timeBounds, testNamespace, backupFolder).
		Return(mocks.backupHandler, nil)

	mocks.backupHandler.EXPECT().
		Wait(gomock.Any()).
		Return(nil)

	mocks.backupHandler.EXPECT().
		GetStats().
		Return(backupStats)

	mocks.backendService.EXPECT(). // backup succeeded, but could not write metadata => delete all
					WriteBackupMetadata(gomock.Any(), routineName, backupFolder, gomock.Any()).
					Return(metadataError)

	mocks.backendService.EXPECT().
		Delete(gomock.Any(), routineName, timestampPath).
		Return(nil)

	handler := runner.Run(ctx, client, backupRoutine, jobTypeFull, testNamespace, now, timeBounds)

	require.NotNil(t, handler)
	err := handler.Wait(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not write backup metadata")
}

func TestRetryableBackupHandler_Cancel(t *testing.T) {
	now := time.UnixMilli(123456789000)
	backupFolder := "test-routine/backup/123456789000/data/test-ns"
	timestampPath := "test-routine/backup/123456789000"

	mocks, runner := initMocks(t)
	defer mocks.ctrl.Finish()

	client := &backup.Client{}
	backupRoutine := &model.BackupRoutine{}
	timeBounds := model.TimeBounds{}

	var wg sync.WaitGroup
	wg.Add(1)

	// Set up minimal expectations to create a handler
	mocks.backupExecutor.EXPECT().
		Run(gomock.Any(), client, backupRoutine, timeBounds, testNamespace, backupFolder).
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

	mocks.backendService.EXPECT(). // backup cancelled => delete all
					Delete(gomock.Any(), routineName, timestampPath).
					Return(nil)

	handler := runner.Run(ctx, client, backupRoutine, jobTypeFull, testNamespace, now, timeBounds)

	// Wait for the handler.Wait to be called
	wg.Wait()

	// Cancel the handler
	handler.Cancel()

	// Assert - the Wait method should eventually return with context.Canceled
	err := handler.Wait(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "context canceled")
}
