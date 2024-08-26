package service

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/pkg/util"
	"github.com/aerospike/backup-go"
	"github.com/aws/smithy-go/ptr"
	"github.com/stretchr/testify/assert"
)

var restoreService = makeTestRestoreService()
var validBackupPath = "./testout/backup/data/namespace"

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
	config := dto.NewConfigWithDefaultValues()
	config.Storage["s"] = &dto.Storage{
		Path: ptr.String("/"),
		Type: dto.Local,
	}
	config.BackupRoutines = map[string]*dto.BackupRoutine{
		"routine": {
			Storage: "s",
		},
		"routine_fail_restore": {
			Storage: "s",
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

func (m *BackendMock) FindIncrementalBackupsForNamespace(_ *dto.TimeBounds, _ string,
) ([]dto.BackupDetails, error) {
	return []dto.BackupDetails{{
		BackupMetadata: dto.BackupMetadata{
			Created:   time.UnixMilli(10),
			Namespace: "ns1",
		},
		Key: ptr.String("key"),
	}, {
		BackupMetadata: dto.BackupMetadata{
			Created:   time.UnixMilli(20),
			Namespace: "ns1",
		},
		Key: ptr.String("key2"),
	}}, nil
}

func (m *BackendMock) ReadClusterConfiguration(_ string) ([]byte, error) {
	return []byte{}, nil
}

func (*BackendMock) FullBackupList(_ *dto.TimeBounds) ([]dto.BackupDetails, error) {
	return []dto.BackupDetails{{
		BackupMetadata: dto.BackupMetadata{
			Created:   time.UnixMilli(5),
			Namespace: "ns1",
		},
		Key: &validBackupPath,
	}}, nil
}

func (*BackendMock) IncrementalBackupList(_ *dto.TimeBounds) ([]dto.BackupDetails, error) {
	return []dto.BackupDetails{{
		BackupMetadata: dto.BackupMetadata{
			Created:   time.UnixMilli(10),
			Namespace: "ns1",
		},
		Key: ptr.String("key"),
	}, {
		BackupMetadata: dto.BackupMetadata{
			Created:   time.UnixMilli(20),
			Namespace: "ns1",
		},
		Key: ptr.String("key2"),
	}}, nil
}

func (*BackendMock) FindLastFullBackup(t time.Time) ([]dto.BackupDetails, error) {
	created := time.UnixMilli(5)

	if t.After(created) {
		return []dto.BackupDetails{{
			BackupMetadata: dto.BackupMetadata{
				Created:   created,
				Namespace: "ns1",
			},
			Key: &validBackupPath,
		}}, nil
	}

	return nil, errBackupNotFound
}

func (*BackendFailMock) FindLastFullBackup(_ time.Time) ([]dto.BackupDetails, error) {
	return nil, errBackupNotFound
}

type BackendFailMock struct {
}

func (m *BackendFailMock) FindIncrementalBackupsForNamespace(_ *dto.TimeBounds, _ string,
) ([]dto.BackupDetails, error) {
	return nil, nil
}

func (m *BackendFailMock) ReadClusterConfiguration(_ string) ([]byte, error) {
	return nil, errors.New("mock error")
}

func (*BackendFailMock) FullBackupList(_ *dto.TimeBounds) ([]dto.BackupDetails, error) {
	return nil, errors.New("mock error")
}

func (*BackendFailMock) IncrementalBackupList(_ *dto.TimeBounds) ([]dto.BackupDetails, error) {
	return nil, errors.New("mock error")
}

func TestRestoreOK(t *testing.T) {
	makeTestFolders()
	t.Cleanup(func() {
		cleanTestFolder()
	})
	restoreRequest := &dto.RestoreRequest{
		DestinationCuster: dto.NewLocalAerospikeCluster(),
		Policy: &dto.RestorePolicy{
			SetList: []string{"set1"},
		},
		SourceStorage: &dto.Storage{
			Path: &validBackupPath,
			Type: dto.Local,
		},
	}
	requestInternal := &dto.RestoreRequestInternal{
		RestoreRequest: *restoreRequest,
		Dir:            &validBackupPath,
	}
	jobID, err := restoreService.Restore(requestInternal)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	jobStatus, _ := restoreService.JobStatus(jobID)
	if jobStatus.Status != dto.JobStatusRunning {
		t.Errorf("Expected jobStatus to be %s, but was %s", dto.JobStatusDone, jobStatus.Status)
	}
	time.Sleep(1 * time.Second)
	jobStatus, _ = restoreService.JobStatus(jobID)
	if jobStatus.Status != dto.JobStatusDone {
		t.Errorf("Expected jobStatus to be %s, but was %s", dto.JobStatusDone, jobStatus.Status)
	}
}

func TestLatestFullBackupBeforeTime(t *testing.T) {
	backupList := []dto.BackupDetails{
		{BackupMetadata: dto.BackupMetadata{Created: time.UnixMilli(10)}},
		{BackupMetadata: dto.BackupMetadata{Created: time.UnixMilli(20)}}, // Should be the latest full backup
		{BackupMetadata: dto.BackupMetadata{Created: time.UnixMilli(20)}}, // Should be the latest full backup too
		{BackupMetadata: dto.BackupMetadata{Created: time.UnixMilli(30)}},
	}

	result := latestFullBackupBeforeTime(backupList, time.UnixMilli(25))

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
	backupList := []dto.BackupDetails{
		{BackupMetadata: dto.BackupMetadata{Created: time.UnixMilli(10)}},
		{BackupMetadata: dto.BackupMetadata{Created: time.UnixMilli(20)}},
		{BackupMetadata: dto.BackupMetadata{Created: time.UnixMilli(30)}},
	}

	result := latestFullBackupBeforeTime(backupList, time.UnixMilli(5))

	if result != nil {
		t.Errorf("Expected a non result, but got %+v", result)
	}
}

func Test_RestoreTimestamp(t *testing.T) {
	request := dto.RestoreTimestampRequest{
		DestinationCuster: dto.NewLocalAerospikeCluster(),
		Policy: &dto.RestorePolicy{
			SetList: []string{"set1"},
		},
		Time:    100,
		Routine: "routine",
	}

	jobID, err := restoreService.RestoreByTime(&request)
	if err != nil {
		t.Errorf("expected nil, got %s", err.Error())
	}

	time.Sleep(1 * time.Second)
	jobStatus, _ := restoreService.JobStatus(jobID)
	if jobStatus.Status != dto.JobStatusDone {
		t.Errorf("Expected jobStatus to be %s, but was %s", dto.JobStatusDone, jobStatus.Status)
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
	requestInternal := &dto.RestoreRequestInternal{
		RestoreRequest: dto.RestoreRequest{
			SourceStorage: &dto.Storage{
				Type: dto.Local,
				Path: util.Ptr("wrongPath"),
			},
		},
		Dir: util.Ptr("wrongPath"),
	}

	_, err := restoreService.Restore(requestInternal)
	if !os.IsNotExist(err) {
		t.Errorf("Expected not exist, but got %v", err)
	}
}

func Test_RestoreFromEmptyFolder(t *testing.T) {
	requestInternal := &dto.RestoreRequestInternal{
		RestoreRequest: dto.RestoreRequest{
			SourceStorage: &dto.Storage{
				Type: dto.Local,
				Path: util.Ptr("./"),
			},
		},
		Dir: util.Ptr("./"),
	}

	_, err := restoreService.Restore(requestInternal)
	if err == nil || !strings.Contains(err.Error(), "no backup files found") {
		t.Errorf("Expected no backup found, but got %v", err)
	}
}

func Test_RestoreFail(t *testing.T) {
	makeTestFolders()
	t.Cleanup(func() {
		cleanTestFolder()
	})

	restoreRequest := &dto.RestoreRequest{
		DestinationCuster: &dto.AerospikeCluster{},
		Policy: &dto.RestorePolicy{
			SetList: []string{"set1"},
		},
		SourceStorage: &dto.Storage{
			Path: &validBackupPath,
		},
	}
	requestInternal := &dto.RestoreRequestInternal{
		RestoreRequest: *restoreRequest,
		Dir:            &validBackupPath,
	}

	jobID, err := restoreService.Restore(requestInternal)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	time.Sleep(1 * time.Second)
	status, _ := restoreService.JobStatus(jobID)
	if status.Status != dto.JobStatusFailed {
		t.Errorf("Expected restore job status to be Failed, but got %s", status.Status)
	}
}

func Test_RestoreByTimeFailNoBackend(t *testing.T) {
	request := &dto.RestoreTimestampRequest{
		Routine: "wrongRoutine",
	}

	_, err := restoreService.RestoreByTime(request)
	if err == nil || !errors.Is(err, errBackendNotFound) {
		t.Errorf("Expected error %v, but got %v", errBackendNotFound, err)
	}
}

func Test_RestoreByTimeFailNoTimestamp(t *testing.T) {
	request := &dto.RestoreTimestampRequest{
		Routine: "routine",
	}

	_, err := restoreService.RestoreByTime(request)
	if err == nil || !errors.Is(err, errBackupNotFound) {
		t.Errorf("Expected error %v, but got %v", errBackupNotFound, err)
	}
}

func Test_RestoreByTimeFailNoBackup(t *testing.T) {
	request := &dto.RestoreTimestampRequest{
		Routine: "routine",
		Time:    1,
	}

	_, err := restoreService.RestoreByTime(request)
	if err == nil || !errors.Is(err, errBackupNotFound) {
		t.Errorf("Expected error %v, but got %v", errBackupNotFound, err)
	}
}

func Test_restoreTimestampFail(t *testing.T) {
	request := &dto.RestoreTimestampRequest{
		Routine:           "routine_fail_restore",
		Time:              10,
		DestinationCuster: &dto.AerospikeCluster{},
	}

	jobID, _ := restoreService.RestoreByTime(request)
	time.Sleep(1 * time.Second)
	status, _ := restoreService.JobStatus(jobID)
	if status.Status != dto.JobStatusFailed {
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

func (m *MockClientManager) GetClient(string) (*backup.Client, error) {
	return &backup.Client{}, nil
}

func (m *MockClientManager) Close(*backup.Client) {
}

func (m *MockClientManager) CreateClient(cluster *dto.AerospikeCluster) (*backup.Client, error) {
	if len(cluster.ASClientHosts()) == 0 {
		return nil, errors.New("no hosts provided")
	}

	return &backup.Client{}, nil
}
