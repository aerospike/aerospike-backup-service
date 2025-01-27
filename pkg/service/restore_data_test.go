package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/stretchr/testify/require"
)

var restoreService = makeTestRestoreService(nil)
var validBackupPath = "testout/backup/data"

type BackendHolderMock struct{}

func (b *BackendHolderMock) GetReader(name string) (BackupMetadataReader, bool) {
	switch name {
	case "routine":
		return &BackendMock{}, true
	case "routine_fail_read":
		return &BackendFailMock{}, true
	case "routine_fail_restore":
		return &BackendFailMock{}, true
	}
	return nil, false
}

func (b *BackendHolderMock) Get(_ string) (BackupMetadataReaderWriter, bool) {
	return nil, false
}

func (b *BackendHolderMock) GetAllReaders() map[string]BackupMetadataReader {
	return nil
}

func (b *BackendHolderMock) Init(_ map[string]*model.BackupRoutine) {
}

func makeTestRestoreService(wg *sync.WaitGroup) *dataRestorer {
	return &dataRestorer{
		configRetriever: configRetriever{
			backends: &BackendHolderMock{},
		},
		restoreJobs:    NewRestoreJobsHolder(),
		restoreService: NewRestoreMock(wg),
		backends:       &BackendHolderMock{},
		clientManager:  &MockClientManager{},
		nsValidator:    &nsValidatorMock{},
	}
}

type BackendMock struct {
}

func (m *BackendMock) FindLastRun(_ context.Context) *model.LastBackupRun {
	return nil
}

func (m *BackendMock) FindIncrementalBackupsForNamespace(_ context.Context, _ model.TimeBounds, _ string,
) ([]model.BackupDetails, error) {
	return []model.BackupDetails{{
		BackupMetadata: model.BackupMetadata{
			Created:   time.UnixMilli(10),
			Namespace: "ns1",
			FileCount: 1,
		},
		Key: "key",
	}, {
		BackupMetadata: model.BackupMetadata{
			Created:   time.UnixMilli(20),
			Namespace: "ns1",
			FileCount: 1,
		},
		Key: "key2",
	}}, nil
}

func (m *BackendMock) ReadClusterConfiguration(_ string) ([]byte, error) {
	return []byte{}, nil
}

func (*BackendMock) FullBackupList(_ context.Context, _ model.TimeBounds) ([]model.BackupDetails, error) {
	return []model.BackupDetails{{
		BackupMetadata: model.BackupMetadata{
			Created:   time.UnixMilli(5),
			Namespace: "ns1",
			FileCount: 1,
		},
		Key: validBackupPath,
	}}, nil
}

func (*BackendMock) IncrementalBackupList(_ context.Context, _ model.TimeBounds) ([]model.BackupDetails, error) {
	return []model.BackupDetails{{
		BackupMetadata: model.BackupMetadata{
			Created:   time.UnixMilli(10),
			Namespace: "ns1",
			FileCount: 1,
		},
		Key: "key",
	}, {
		BackupMetadata: model.BackupMetadata{
			Created:   time.UnixMilli(20),
			Namespace: "ns1",
			FileCount: 1,
		},
		Key: "key2",
	}}, nil
}

func (*BackendMock) FindLastFullBackup(t time.Time) ([]model.BackupDetails, error) {
	created := time.UnixMilli(5)

	if t.After(created) {
		return []model.BackupDetails{{
			BackupMetadata: model.BackupMetadata{
				Created:   created,
				Namespace: "ns1",
				FileCount: 1,
			},
			Key: validBackupPath,
		}}, nil
	}

	return nil, errBackupNotFound
}

func (*BackendFailMock) FindLastFullBackup(_ time.Time) ([]model.BackupDetails, error) {
	return nil, errBackupNotFound
}

type BackendFailMock struct {
}

func (m *BackendFailMock) FindLastRun(_ context.Context) *model.LastBackupRun {
	return nil
}

func (m *BackendFailMock) FindIncrementalBackupsForNamespace(_ context.Context, _ model.TimeBounds, _ string,
) ([]model.BackupDetails, error) {
	return nil, nil
}

func (m *BackendFailMock) ReadClusterConfiguration(_ string) ([]byte, error) {
	return nil, errors.New("mock error")
}

func (*BackendFailMock) FullBackupList(_ context.Context, _ model.TimeBounds) ([]model.BackupDetails, error) {
	return nil, errors.New("mock error")
}

func (*BackendFailMock) IncrementalBackupList(_ context.Context, _ model.TimeBounds) ([]model.BackupDetails, error) {
	return nil, errors.New("mock error")
}

type nsValidatorMock struct {
	isEmpty bool
}

func (n nsValidatorMock) MissingNamespaces(*model.AerospikeCluster, []string) []string {
	return nil
}

func (n nsValidatorMock) ValidateRoutines(*model.AerospikeCluster, map[string]*model.BackupRoutine) error {
	return nil
}

func (n nsValidatorMock) IsEmpty(backup.AerospikeClient, string, []string) (bool, error) {
	return n.isEmpty, nil
}

func TestRestoreOK(t *testing.T) {
	restoreRequest := &model.RestoreRequest{
		DestinationCluster: model.NewLocalAerospikeCluster(),
		Policy:             &model.RestorePolicy{},
		SourceStorage: &model.LocalStorage{
			Path: validBackupPath,
		},
		BackupDataPath: "namespace",
	}
	jobID, err := restoreService.Restore(restoreRequest)
	require.NoError(t, err)
	jobStatus, err := restoreService.JobStatus(jobID)
	require.NoError(t, err)
	require.Equal(t, model.JobStatusRunning, jobStatus.Status)

	_, err = waitForJobStatus(t, jobID, model.JobStatusDone)
	require.NoError(t, err)
}

func TestLatestFullBackupBeforeTime(t *testing.T) {
	backupList := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(10)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(20)}}, // Should be the latest full backup
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(20)}}, // Should be the latest full backup too
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(30)}},
	}

	toTime := time.UnixMilli(25)
	result := latestBackupBeforeTime(backupList, &toTime)

	require.NotNil(t, result)
	require.Equal(t, 2, len(result))
	require.Equal(t, result[0], backupList[1])
}

func TestLatestFullBackupEqualTime(t *testing.T) {
	backupList := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(10)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(20)}}, // Should be the latest full backup
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(30)}},
	}

	toTime := time.UnixMilli(20)
	result := latestBackupBeforeTime(backupList, &toTime)

	require.NotNil(t, result)
	require.Equal(t, 1, len(result))
	require.Equal(t, result[0], backupList[1])
}

func TestLatestFullBackupBeforeTime_NotFound(t *testing.T) {
	backupList := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(10)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(20)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(30)}},
	}

	toTime := time.UnixMilli(5)
	result := latestBackupBeforeTime(backupList, &toTime)

	require.Nil(t, result)
}

func Test_RestoreTimestamp(t *testing.T) {
	request := model.RestoreTimestampRequest{
		DestinationCluster: model.NewLocalAerospikeCluster(),
		Policy:             &model.RestorePolicy{},
		Time:               time.UnixMilli(100),
		RoutineName:        "routine",
	}

	jobID, err := restoreService.RestoreByTime(&request)
	require.NoError(t, err, "RestoreByTime should not return an error")

	jobStatus, err := waitForJobStatus(t, jobID, model.JobStatusDone)
	require.NoError(t, err)
	require.Equal(t, 3, int(jobStatus.ReadRecords), "Expected 3 (one full and 2 incremental backups)")
}

func Test_WrongStatus(t *testing.T) {
	_, err := restoreService.JobStatus(1111)
	require.Error(t, err)
}

func Test_RestoreByTimeFailNoBackend(t *testing.T) {
	request := &model.RestoreTimestampRequest{
		RoutineName: "wrongRoutine",
	}

	_, err := restoreService.RestoreByTime(request)
	require.ErrorIs(t, err, errBackendNotFound)
}

func Test_RestoreByTimeFailNoTimestamp(t *testing.T) {
	request := &model.RestoreTimestampRequest{
		RoutineName: "routine",
	}

	_, err := restoreService.RestoreByTime(request)
	require.ErrorIs(t, err, errBackupNotFound)
}

func Test_RestoreByTimeFailNoBackup(t *testing.T) {
	request := &model.RestoreTimestampRequest{
		RoutineName: "routine",
		Time:        time.UnixMilli(1),
	}

	_, err := restoreService.RestoreByTime(request)
	require.ErrorIs(t, err, errBackupNotFound)
}

func Test_restoreTimestampFail(t *testing.T) {
	request := &model.RestoreTimestampRequest{
		RoutineName:        "routine_fail_restore",
		Time:               time.UnixMilli(10),
		DestinationCluster: &model.AerospikeCluster{},
	}

	_, err := restoreService.RestoreByTime(request)
	require.Error(t, err)
}

// MockClientManager is a mock implementation of ClientManager for testing.
type MockClientManager struct {
}

func (m *MockClientManager) GetClient(_ *model.AerospikeCluster) (*backup.Client, error) {
	return &backup.Client{}, nil
}

func (m *MockClientManager) Close(*backup.Client) {
}

func (m *MockClientManager) CreateClient(cluster *model.AerospikeCluster) (*backup.Client, error) {
	if len(cluster.ASClientHosts()) == 0 {
		return nil, errors.New("no hosts provided")
	}

	return &backup.Client{}, nil
}

func TestRestoreCancel(t *testing.T) {
	wg := &sync.WaitGroup{}
	restoreService = makeTestRestoreService(wg)
	restoreRequest := &model.RestoreRequest{
		DestinationCluster: model.NewLocalAerospikeCluster(),
		Policy:             &model.RestorePolicy{},
		SourceStorage: &model.LocalStorage{
			Path: validBackupPath,
		},
		BackupDataPath: "namespace",
	}
	wg.Add(1)
	NewRestoreMock(nil).restoreWaitWg = wg
	jobID, err := restoreService.Restore(restoreRequest)
	require.NoError(t, err)

	wg.Wait() // wait until restore starts
	err = restoreService.CancelRestore(jobID)
	require.NoError(t, err)
	_, err = waitForJobStatus(t, jobID, model.JobStatusCancelled)
	require.NoError(t, err)
}

func waitForJobStatus(
	t *testing.T, jobID model.RestoreJobID, expected model.JobStatus,
) (*model.RestoreJobStatus, error) {
	t.Helper()
	return wait(func() (*model.RestoreJobStatus, bool) {
		status, err := restoreService.JobStatus(jobID)
		require.NoError(t, err)
		require.NotNil(t, status)
		return status, status.Status == expected
	})
}

func wait[T any](f func() (T, bool)) (T, error) {
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		val, success := f()
		if success {
			return val, nil
		}

		time.Sleep(20 * time.Millisecond)
	}

	var result T
	return result, errors.New("timeout reached")
}
