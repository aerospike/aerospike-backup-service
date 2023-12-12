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
	// Create a sample list of BackupDetails
	backupList := []model.BackupDetails{
		{LastModified: util.Ptr(time.UnixMilli(10))},
		{LastModified: util.Ptr(time.UnixMilli(20))}, // Should be the latest full backup
		{LastModified: util.Ptr(time.UnixMilli(30))},
	}

	result := latestFullBackupBeforeTime(backupList, time.UnixMilli(25))

	// Check if the result is not nil (i.e., a backup was found)
	if result == nil {
		t.Error("Expected a non-nil result, but got nil")
	}

	// Check if the result is the expected backup
	if result != &backupList[1] {
		t.Errorf("Expected the latest backup, but got %+v", result)
	}
}
