package service

import (
	"os"
	"testing"
	"time"

	"github.com/aws/smithy-go/ptr"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
)

func TestRestoreMemory(t *testing.T) {
	restoreService := NewRestoreMemory(nil, nil)
	restoreRequest := &model.RestoreRequest{
		DestinationCuster: &model.AerospikeCluster{
			Host: util.Ptr("localhost"),
			Port: util.Ptr(int32(3000)),
		},
		Policy: &model.RestorePolicy{
			SetList: []string{"set1"},
		},
		SourceStorage: &model.Storage{},
	}
	path := "./testout/backup"
	requestInternal := &model.RestoreRequestInternal{
		RestoreRequest: *restoreRequest,
		Dir:            util.Ptr(path),
	}
	err := createMockBackupFile(path)
	jobID, err := restoreService.Restore(requestInternal)

	jobStatus, _ := restoreService.JobStatus(jobID)
	if jobStatus.Status != model.JobStatusRunning {
		t.Errorf("Expected jobStatus to be %s", model.JobStatusRunning)
	}

	time.Sleep(1 * time.Second)
	// jobStatus, _ = restoreService.JobStatus(jobID)
	// if jobStatus.Status != model.JobStatusDone {
	// 	t.Errorf("Expected jobStatus to be %s, but was %s", model.JobStatusDone, jobStatus.Status)
	// }

	wrongJobStatus, err := restoreService.JobStatus(1111)
	if err == nil {
		t.Errorf("Expected not found, but go %v", wrongJobStatus)
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

	if result != &backupList[1] {
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

func (*BackendMock) FullBackupList(_ int64, _ int64) ([]model.BackupDetails, error) {
	return []model.BackupDetails{{
		BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(5)},
		Key:            ptr.String("key"),
	}}, nil
}

func (*BackendMock) IncrementalBackupList() ([]model.BackupDetails, error) {
	return []model.BackupDetails{{
		BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(10)},
		Key:            ptr.String("key"),
	}, {
		BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(20)},
		Key:            ptr.String("key2"),
	}}, nil
}

func TestRestoreTimestamp(t *testing.T) {
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
	restoreService := NewRestoreMemory(backends, config)

	request := model.RestoreTimestampRequest{
		DestinationCuster: &model.AerospikeCluster{
			Host: util.Ptr("localhost"),
			Port: util.Ptr(int32(3000)),
		},
		Policy: &model.RestorePolicy{
			SetList: []string{"set1"},
		},
		Time:    100,
		Routine: "routine",
	}

	_, err := restoreService.RestoreByTime(&request)
	if err != nil {
		t.Errorf("expected nil, got %s", err.Error())
	}

	// time.Sleep(1 * time.Second)
	// jobStatus, _ := restoreService.JobStatus(jobID)
	// if jobStatus.Status != model.JobStatusDone {
	// 	t.Errorf("Expected jobStatus to be %s, but was %s", model.JobStatusDone, jobStatus.Status)
	// }
	// if jobStatus.TotalRecords != 3 {
	// 	t.Errorf("Expected 3 (one full and 2 incremental backups")
	// }
}
