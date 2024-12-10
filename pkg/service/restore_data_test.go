package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/stretchr/testify/require"
)

var restoreService = makeTestRestoreService()
var validBackupPath = "./testout/backup/data"

func makeTestFolders() {
	_ = os.MkdirAll(validBackupPath, os.ModePerm)
	create, _ := os.Create(validBackupPath + "/backup.asb")
	_ = create.Close()
}

func cleanTestFolder() {
	_ = os.RemoveAll("./testout")
}

type BackendHolderMock struct{}

func (b *BackendHolderMock) GetReader(name string) (BackupListReader, bool) {
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

func (b *BackendHolderMock) Get(_ string) (*BackupBackend, bool) {
	return nil, false
}

func (b *BackendHolderMock) GetAllReaders() map[string]BackupListReader {
	return nil
}

func (b *BackendHolderMock) Init(_ *model.Config) {
}

func makeTestRestoreService() *dataRestorer {
	storage := &model.LocalStorage{}
	config := model.NewConfig()
	_ = config.AddStorage("s", storage)
	config.BackupRoutines = map[string]*model.BackupRoutine{
		"routine": {
			Storage: storage,
		},
		"routine_fail_restore": {
			Storage: storage,
		},
	}

	return &dataRestorer{
		configRetriever: configRetriever{
			backends: &BackendHolderMock{},
		},
		restoreJobs:    NewRestoreJobsHolder(),
		restoreService: NewRestoreMock(),
		backends:       &BackendHolderMock{},
		config:         config,
		clientManager:  &MockClientManager{},
	}
}

type BackendMock struct {
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

func TestRestoreOK(t *testing.T) {
	makeTestFolders()
	t.Cleanup(func() {
		cleanTestFolder()
	})
	restoreRequest := &model.RestoreRequest{
		DestinationCluster: model.NewLocalAerospikeCluster(),
		Policy: &model.RestorePolicy{
			SetList: []string{"set1"},
		},
		SourceStorage: &model.LocalStorage{
			Path: validBackupPath,
		},
		BackupDataPath: "namespace",
	}
	jobID, err := restoreService.Restore(restoreRequest)
	require.NoError(t, err)
	jobStatus, err := restoreService.JobStatus(jobID)
	require.NoError(t, err)
	if jobStatus.Status != model.JobStatusRunning {
		t.Errorf("Expected jobStatus to be %s, but was %s", model.JobStatusDone, jobStatus.Status)
	}
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		jobStatus, err = restoreService.JobStatus(jobID)
		require.NoError(t, err)

		if jobStatus.Status == model.JobStatusDone {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	require.Equal(t, model.JobStatusDone, jobStatus.Status)
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

	if result == nil {
		t.Error("Expected a non-nil result, but got nil")
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 backups")
	}
	if result[0] != backupList[1] {
		t.Errorf("Expected the latest backup, but got %+v", result)
	}
}

func TestLatestFullBackupEqualTime(t *testing.T) {
	backupList := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(10)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(20)}}, // Should be the latest full backup
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(30)}},
	}

	toTime := time.UnixMilli(20)
	result := latestBackupBeforeTime(backupList, &toTime)

	if result == nil {
		t.Error("Expected a non-nil result, but got nil")
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 backup")
	}
	if result[0] != backupList[1] {
		t.Errorf("Expected the latest backup, but got %+v", result)
	}
}

func TestLatestFullBackupBeforeTime_NotFound(t *testing.T) {
	backupList := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(10)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(20)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(30)}},
	}

	toTime := time.UnixMilli(5)
	result := latestBackupBeforeTime(backupList, &toTime)

	if result != nil {
		t.Errorf("Expected a non result, but got %+v", result)
	}
}

func Test_RestoreTimestamp(t *testing.T) {
	request := model.RestoreTimestampRequest{
		DestinationCluster: model.NewLocalAerospikeCluster(),
		Policy: &model.RestorePolicy{
			SetList: []string{"set1"},
		},
		Time:        time.UnixMilli(100),
		RoutineName: "routine",
	}

	jobID, err := restoreService.RestoreByTime(&request)
	require.NoError(t, err, "RestoreByTime should not return an error")

	deadline := time.Now().Add(1 * time.Second)
	var jobStatus *model.RestoreJobStatus
	for time.Now().Before(deadline) {
		jobStatus, err = restoreService.JobStatus(jobID)
		require.NoError(t, err)

		if jobStatus.Status == model.JobStatusDone {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	require.NotNil(t, jobStatus)
	require.Equal(t, model.JobStatusDone, jobStatus.Status)
	require.Equal(t, 3, int(jobStatus.ReadRecords), "Expected 3 (one full and 2 incremental backups)")
}

func Test_WrongStatus(t *testing.T) {
	wrongJobStatus, err := restoreService.JobStatus(1111)
	if err == nil {
		t.Errorf("Expected not found, but got %v", wrongJobStatus)
	}
}

func Test_RestoreByTimeFailNoBackend(t *testing.T) {
	request := &model.RestoreTimestampRequest{
		RoutineName: "wrongRoutine",
	}

	_, err := restoreService.RestoreByTime(request)
	if err == nil || !errors.Is(err, errBackendNotFound) {
		t.Errorf("Expected error %v, but got %v", errBackendNotFound, err)
	}
}

func Test_RestoreByTimeFailNoTimestamp(t *testing.T) {
	request := &model.RestoreTimestampRequest{
		RoutineName: "routine",
	}

	_, err := restoreService.RestoreByTime(request)
	if err == nil || !errors.Is(err, errBackupNotFound) {
		t.Errorf("Expected error %v, but got %v", errBackupNotFound, err)
	}
}

func Test_RestoreByTimeFailNoBackup(t *testing.T) {
	request := &model.RestoreTimestampRequest{
		RoutineName: "routine",
		Time:        time.UnixMilli(1),
	}

	_, err := restoreService.RestoreByTime(request)
	if err == nil || !errors.Is(err, errBackupNotFound) {
		t.Errorf("Expected error %v, but got %v", errBackupNotFound, err)
	}
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

// MockClientManager is a mock implementation of ClientManager for testing
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
