package service

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

const tempFolder = "./tmp"

func TestFullBackupRemoveFiles(t *testing.T) {
	backend := &BackupBackend{
		storage:     &model.LocalStorage{Path: tempFolder},
		routineName: "routine",
	}

	path := backend.routineName + "/backup/data/source-ns1/"
	_ = os.MkdirAll(path, 0744)
	_ = backend.writeBackupMetadata(context.Background(), path, model.BackupMetadata{Created: time.UnixMilli(10)})

	to := model.NewTimeBoundsTo(time.UnixMilli(1000))
	list, _ := backend.FullBackupList(context.Background(), to)
	if len(list) != 1 {
		t.Errorf("Expected list size 1, got %v", list)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempFolder)
	})
}

func TestFullBackupKeepFiles(t *testing.T) {
	backend := &BackupBackend{
		storage:     &model.LocalStorage{Path: tempFolder},
		routineName: "routine",
	}

	for _, t := range []int64{10, 20, 30} {
		path := backend.routineName + "/backup/" + strconv.FormatInt(t, 10) + "/data/source-ns1/"
		_ = os.MkdirAll(path, 0744)
		_ = backend.writeBackupMetadata(context.Background(), path, model.BackupMetadata{Created: time.UnixMilli(t)})
	}

	bounds := model.NewTimeBoundsTo(time.UnixMilli(25))
	list, _ := backend.FullBackupList(context.Background(), bounds)
	if len(list) != 2 {
		t.Errorf("Expected list size 2, got %v", list)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempFolder)
	})
}
