package service

import (
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

const tempFolder = "./tmp"

func TestFullBackupRemoveFiles(t *testing.T) {
	backend := &BackupBackend{
		StorageAccessor:      &OSDiskAccessor{},
		fullBackupsPath:      tempFolder + "/routine/backup",
		removeFullBackup:     true,
		fullBackupInProgress: &atomic.Bool{},
	}

	path := backend.fullBackupsPath + "/data/source-ns1/"
	_ = os.MkdirAll(path, 0744)
	_ = backend.writeBackupMetadata(path, model.BackupMetadata{Created: time.UnixMilli(10)})

	to := model.NewTimeBoundsTo(time.UnixMilli(1000))
	list, _ := backend.FullBackupList(to)
	if len(list) != 1 {
		t.Errorf("Expected list size 1, got %v", list)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempFolder)
	})
}

func TestFullBackupKeepFiles(t *testing.T) {
	backend := &BackupBackend{
		StorageAccessor:      &OSDiskAccessor{},
		fullBackupsPath:      tempFolder + "/routine/backup",
		removeFullBackup:     false,
		fullBackupInProgress: &atomic.Bool{},
	}

	for _, t := range []int64{10, 20, 30} {
		path := backend.fullBackupsPath + "/" + strconv.FormatInt(t, 10) + "/data/source-ns1/"
		_ = os.MkdirAll(path, 0744)
		_ = backend.writeBackupMetadata(path, model.BackupMetadata{Created: time.UnixMilli(t)})
	}

	bounds := model.NewTimeBoundsTo(time.UnixMilli(25))
	list, _ := backend.FullBackupList(bounds)
	if len(list) != 2 {
		t.Errorf("Expected list size 2, got %v", list)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempFolder)
	})
}
