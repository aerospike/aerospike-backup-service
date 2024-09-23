package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/stretchr/testify/assert"
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
		return &BackendMock{}, true
	}
	return nil, false
}

func (b *BackendHolderMock) Get(_ string) (*BackupBackend, bool) {
	return nil, false
}

func (b *BackendHolderMock) SetData(_ map[string]*BackupBackend) {
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
		restoreJobs:    NewJobsHolder(),
		restoreService: NewRestoreMock(),
		backends:       &BackendHolderMock{},
		config:         config,
		clientManager:  &MockClientManager{},
	}
}

type BackendMock struct {
}

func (m *BackendMock) FindIncrementalBackupsForNamespace(_ context.Context, _ *model.TimeBounds, _ string,
) ([]model.BackupDetails, error) {
	return []model.BackupDetails{{
		BackupMetadata: model.BackupMetadata{
			Created:   time.UnixMilli(10),
			Namespace: "ns1",
		},
		Key: "key",
	}, {
		BackupMetadata: model.BackupMetadata{
			Created:   time.UnixMilli(20),
			Namespace: "ns1",
		},
		Key: "key2",
	}}, nil
}

func (m *BackendMock) ReadClusterConfiguration(_ string) ([]byte, error) {
	return []byte{}, nil
}

func (*BackendMock) FullBackupList(_ context.Context, _ *model.TimeBounds) ([]model.BackupDetails, error) {
	return []model.BackupDetails{{
		BackupMetadata: model.BackupMetadata{
			Created:   time.UnixMilli(5),
			Namespace: "ns1",
		},
		Key: validBackupPath,
	}}, nil
}

func (*BackendMock) IncrementalBackupList(_ context.Context, _ *model.TimeBounds) ([]model.BackupDetails, error) {
	return []model.BackupDetails{{
		BackupMetadata: model.BackupMetadata{
			Created:   time.UnixMilli(10),
			Namespace: "ns1",
		},
		Key: "key",
	}, {
		BackupMetadata: model.BackupMetadata{
			Created:   time.UnixMilli(20),
			Namespace: "ns1",
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

func (m *BackendFailMock) FindIncrementalBackupsForNamespace(_ context.Context, _ *model.TimeBounds, _ string,
) ([]model.BackupDetails, error) {
	return nil, nil
}

func (m *BackendFailMock) ReadClusterConfiguration(_ string) ([]byte, error) {
	return nil, errors.New("mock error")
}

func (*BackendFailMock) FullBackupList(_ context.Context, _ *model.TimeBounds) ([]model.BackupDetails, error) {
	return nil, errors.New("mock error")
}

func (*BackendFailMock) IncrementalBackupList(_ context.Context, _ *model.TimeBounds) ([]model.BackupDetails, error) {
	return nil, errors.New("mock error")
}

func TestRestoreOK(t *testing.T) {
	makeTestFolders()
	t.Cleanup(func() {
		cleanTestFolder()
	})
	restoreRequest := &model.RestoreRequest{
		DestinationCuster: model.NewLocalAerospikeCluster(),
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
	time.Sleep(1 * time.Second)
	jobStatus, _ = restoreService.JobStatus(jobID)
	if jobStatus.Status != model.JobStatusDone {
		t.Errorf("Expected jobStatus to be %s, but was %s", model.JobStatusDone, jobStatus.Status)
	}
}

func TestLatestFullBackupBeforeTime(t *testing.T) {
	backupList := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(10)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(20)}}, // Should be the latest full backup
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(20)}}, // Should be the latest full backup too
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(30)}},
	}

	result := latestBackupBeforeTime(backupList, time.UnixMilli(25))

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

func TestLatestFullBackupBeforeTime_NotFound(t *testing.T) {
	backupList := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(10)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(20)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(30)}},
	}

	result := latestBackupBeforeTime(backupList, time.UnixMilli(5))

	if result != nil {
		t.Errorf("Expected a non result, but got %+v", result)
	}
}

func Test_RestoreTimestamp(t *testing.T) {
	request := model.RestoreTimestampRequest{
		DestinationCuster: model.NewLocalAerospikeCluster(),
		Policy: &model.RestorePolicy{
			SetList: []string{"set1"},
		},
		Time:    time.UnixMilli(100),
		Routine: "routine",
	}

	jobID, err := restoreService.RestoreByTime(&request)
	if err != nil {
		t.Errorf("expected nil, got %s", err.Error())
	}

	time.Sleep(1 * time.Second)
	jobStatus, _ := restoreService.JobStatus(jobID)
	if jobStatus.Status != model.JobStatusDone {
		t.Errorf("Expected jobStatus to be %s, but was %s", model.JobStatusDone, jobStatus.Status)
	}
	if jobStatus.ReadRecords != 3 {
		t.Errorf("Expected 3 (one full and 2 incremental backups), got %d", jobStatus.ReadRecords)
	}
}

func Test_WrongStatus(t *testing.T) {
	wrongJobStatus, err := restoreService.JobStatus(1111)
	if err == nil {
		t.Errorf("Expected not found, but got %v", wrongJobStatus)
	}
}

func Test_RestoreFromWrongFolder(t *testing.T) {
	restoreRequest := &model.RestoreRequest{
		DestinationCuster: model.NewLocalAerospikeCluster(),
		Policy: &model.RestorePolicy{
			SetList: []string{"set1"},
		},
		SourceStorage: &model.LocalStorage{
			Path: validBackupPath,
		},
		BackupDataPath: "wrongDir",
	}

	_, err := restoreService.Restore(restoreRequest)
	if !os.IsNotExist(err) {
		t.Errorf("Expected not exist, but got %v", err)
	}
}

func Test_RestoreFromEmptyFolder(t *testing.T) {
	restoreRequest := &model.RestoreRequest{
		DestinationCuster: model.NewLocalAerospikeCluster(),
		Policy: &model.RestorePolicy{
			SetList: []string{"set1"},
		},
		SourceStorage: &model.LocalStorage{
			Path: "./",
		},
		BackupDataPath: "",
	}

	_, err := restoreService.Restore(restoreRequest)
	if err == nil || !strings.Contains(err.Error(), "no backup files found") {
		t.Errorf("Expected no backup found, but got %v", err)
	}
}

func Test_RestoreFail(t *testing.T) {
	makeTestFolders()
	t.Cleanup(func() {
		cleanTestFolder()
	})

	restoreRequest := &model.RestoreRequest{
		DestinationCuster: &model.AerospikeCluster{},
		Policy: &model.RestorePolicy{
			SetList: []string{"set1"},
		},
		SourceStorage: &model.LocalStorage{
			Path: validBackupPath,
		},
		BackupDataPath: "",
	}

	jobID, err := restoreService.Restore(restoreRequest)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	time.Sleep(1 * time.Second)
	status, _ := restoreService.JobStatus(jobID)
	if status.Status != model.JobStatusFailed {
		t.Errorf("Expected restore job status to be Failed, but got %s", status.Status)
	}
}

func Test_RestoreByTimeFailNoBackend(t *testing.T) {
	request := &model.RestoreTimestampRequest{
		Routine: "wrongRoutine",
	}

	_, err := restoreService.RestoreByTime(request)
	if err == nil || !errors.Is(err, errBackendNotFound) {
		t.Errorf("Expected error %v, but got %v", errBackendNotFound, err)
	}
}

func Test_RestoreByTimeFailNoTimestamp(t *testing.T) {
	request := &model.RestoreTimestampRequest{
		Routine: "routine",
	}

	_, err := restoreService.RestoreByTime(request)
	if err == nil || !errors.Is(err, errBackupNotFound) {
		t.Errorf("Expected error %v, but got %v", errBackupNotFound, err)
	}
}

func Test_RestoreByTimeFailNoBackup(t *testing.T) {
	request := &model.RestoreTimestampRequest{
		Routine: "routine",
		Time:    time.UnixMilli(1),
	}

	_, err := restoreService.RestoreByTime(request)
	if err == nil || !errors.Is(err, errBackupNotFound) {
		t.Errorf("Expected error %v, but got %v", errBackupNotFound, err)
	}
}

func Test_restoreTimestampFail(t *testing.T) {
	request := &model.RestoreTimestampRequest{
		Routine:           "routine_fail_restore",
		Time:              time.UnixMilli(10),
		DestinationCuster: &model.AerospikeCluster{},
	}

	jobID, _ := restoreService.RestoreByTime(request)
	time.Sleep(1 * time.Second)
	status, _ := restoreService.JobStatus(jobID)
	if status.Status != model.JobStatusFailed {
		t.Errorf("Expected restore job status to be Failed, but got %s", status.Status)
	}
}

func Test_RetrieveConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		routine   string
		timestamp time.Time
		wantErr   bool
	}{
		{
			name:      "normal",
			routine:   "routine",
			timestamp: time.UnixMilli(10),
			wantErr:   false,
		},
		{
			name:      "wrong time",
			routine:   "routine",
			timestamp: time.UnixMilli(1),
			wantErr:   true,
		},
		{
			name:      "wrong routine",
			routine:   "routine_fail_read",
			timestamp: time.UnixMilli(10),
			wantErr:   true,
		},
		{
			name:      "routine not found",
			routine:   "routine not found",
			timestamp: time.UnixMilli(10),
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := restoreService.RetrieveConfiguration(tt.routine, tt.timestamp)
			assert.Equal(t, tt.wantErr, err != nil, "Unexpected error presence, got: %v", err)

			if !tt.wantErr {
				assert.NotNil(t, res, "Expected non-nil result, got nil.")
			} else {
				assert.Nil(t, res, "Expected nil result as an error was expected.")
			}
		})
	}
}

func Test_CalculateConfigurationBackupPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name:    "NormalPath",
			path:    "backup/12345/data/ns1",
			want:    "backup/12345/configuration",
			wantErr: false,
		},
		{
			name:    "InvalidPath",
			path:    "://",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calculateConfigurationBackupPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("calculateConfigurationBackupPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.want {
				t.Errorf("calculateConfigurationBackupPath() got = %v, want %v", result, tt.want)
			}
		})
	}
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
