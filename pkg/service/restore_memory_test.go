package service

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
	"github.com/aws/smithy-go/ptr"
)

var restoreService *RestoreMemory

var validBackupPath = "./testout/backup"

func TestMain(m *testing.M) {
	config := model.NewConfigWithDefaultValues()
	config.Storage["s"] = &model.Storage{
		Path: ptr.String("/"),
	}
	config.BackupRoutines["routine"] = &model.BackupRoutine{
		Storage: "s",
	}
	backends := map[string]BackupListReader{
		"routine": &BackendMock{},
	}
	restoreService = NewRestoreMemory(backends, config)
	_ = createMockBackupFile(validBackupPath)

	m.Run()
	os.RemoveAll("./testout")
}

func TestRestoreOK(t *testing.T) {
	restoreRequest := &model.RestoreRequest{
		DestinationCuster: model.NewLocalAerospikeCluster(),
		Policy: &model.RestorePolicy{
			SetList: []string{"set1"},
		},
		SourceStorage: &model.Storage{},
	}
	requestInternal := &model.RestoreRequestInternal{
		RestoreRequest: *restoreRequest,
		Dir:            util.Ptr(validBackupPath),
	}
	jobID, _ := restoreService.Restore(requestInternal)

	jobStatus, _ := restoreService.JobStatus(jobID)
	if jobStatus.Status != model.JobStatusRunning {
		t.Errorf("Expected jobStatus to be %s", model.JobStatusRunning)
	}
	jobStatus, _ = restoreService.JobStatus(jobID)
	if jobStatus.Status != model.JobStatusRunning {
		t.Errorf("Expected jobStatus to be %s, but was %s", model.JobStatusDone, jobStatus.Status)
	}
	time.Sleep(1 * time.Second)
	jobStatus, _ = restoreService.JobStatus(jobID)
	if jobStatus.Status != model.JobStatusDone {
		t.Errorf("Expected jobStatus to be %s, but was %s", model.JobStatusDone, jobStatus.Status)
	}
}

func createMockBackupFile(path string) error {
	os.MkdirAll(path, os.ModePerm)
	create, err := os.Create(path + "/backup.asb")
	create.Close()
	return err
}

func TestLatestFullBackupBeforeTime(t *testing.T) {
	backupList := []model.BackupDetails{
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(10)}},
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(20)}}, // Should be the latest full backup
		{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(30)}},
	}

	result := latestFullBackupBeforeTime(backupList, time.UnixMilli(25))

	if result == nil {
		t.Error("Expected a non-nil result, but got nil")
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

	result := latestFullBackupBeforeTime(backupList, time.UnixMilli(5))

	if result != nil {
		t.Errorf("Expected a non result, but got %+v", result)
	}
}

type BackendMock struct {
}

func (*BackendMock) FullBackupList(_ *model.TimeBounds) ([]model.BackupDetails, error) {
	return []model.BackupDetails{{
		BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(5)},
		Key:            ptr.String("key"),
	}}, nil
}

func (*BackendMock) IncrementalBackupList(_ *model.TimeBounds) ([]model.BackupDetails, error) {
	return []model.BackupDetails{{
		BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(10)},
		Key:            ptr.String("key"),
	}, {
		BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(20)},
		Key:            ptr.String("key2"),
	}}, nil
}

func Test_RestoreTimestamp(t *testing.T) {
	request := model.RestoreTimestampRequest{
		DestinationCuster: model.NewLocalAerospikeCluster(),
		Policy: &model.RestorePolicy{
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
	if jobStatus.Status != model.JobStatusDone {
		t.Errorf("Expected jobStatus to be %s, but was %s", model.JobStatusDone, jobStatus.Status)
	}
	if jobStatus.TotalRecords != 3 {
		t.Errorf("Expected 3 (one full and 2 incremental backups")
	}
}

func Test_WrongStatus(t *testing.T) {
	wrongJobStatus, err := restoreService.JobStatus(1111)
	if err == nil {
		t.Errorf("Expected not found, but got %v", wrongJobStatus)
	}
}

func Test_RestoreFromWrongFolder(t *testing.T) {
	requestInternal := &model.RestoreRequestInternal{
		RestoreRequest: model.RestoreRequest{
			SourceStorage: &model.Storage{
				Type: model.Local,
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
	requestInternal := &model.RestoreRequestInternal{
		RestoreRequest: model.RestoreRequest{
			SourceStorage: &model.Storage{
				Type: model.Local,
			},
		},
		Dir: util.Ptr("./"),
	}

	_, err := restoreService.Restore(requestInternal)
	if !strings.Contains(err.Error(), "no backup files found") {
		t.Errorf("Expected no backup found, but got %v", err)
	}
}

func Test_RestoreFail(t *testing.T) {
	restoreRequest := &model.RestoreRequest{
		DestinationCuster: nil,
		Policy: &model.RestorePolicy{
			SetList: []string{"set1"},
		},
		SourceStorage: &model.Storage{},
	}
	requestInternal := &model.RestoreRequestInternal{
		RestoreRequest: *restoreRequest,
		Dir:            util.Ptr(validBackupPath),
	}

	jobId, _ := restoreService.Restore(requestInternal)
	time.Sleep(1 * time.Second)
	status, _ := restoreService.JobStatus(jobId)
	if status.Status != model.JobStatusFailed {
		t.Errorf("Expected restore job status to be Failed, but got %s", status.Status)
	}
}
