package service

import (
	"testing"
	"time"

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
	}
	requestInternal := &model.RestoreRequestInternal{
		RestoreRequest: *restoreRequest,
		Dir:            util.Ptr("./testout/backup"),
	}
	jobID := restoreService.Restore(requestInternal)

	jobStatus := restoreService.JobStatus(jobID)
	if jobStatus != jobStatusRunning {
		t.Errorf("Expected jobStatus to be %s", jobStatusRunning)
	}

	wrongJobStatus := restoreService.JobStatus(1111)
	if wrongJobStatus != jobStatusNA {
		t.Errorf("Expected jobStatus to be %s", jobStatusNA)
	}
}

func TestLatestFullBackupBeforeTime(t *testing.T) {
	backupList := []model.BackupDetails{
		{LastModified: util.Ptr(time.UnixMilli(10))},
		{LastModified: util.Ptr(time.UnixMilli(20))}, // Should be the latest full backup
		{LastModified: util.Ptr(time.UnixMilli(30))},
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
		{LastModified: util.Ptr(time.UnixMilli(10))},
		{LastModified: util.Ptr(time.UnixMilli(20))},
		{LastModified: util.Ptr(time.UnixMilli(30))},
	}

	result := latestFullBackupBeforeTime(backupList, time.UnixMilli(5))

	if result != nil {
		t.Errorf("Expected a non result, but got %+v", result)
	}
}
