package service

import (
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aerospike/backup/pkg/model"
)

const tempFolder = "./tmp"

func TestFullBackupRemoveFiles(t *testing.T) {
	backend := &BackupBackend{
		StorageAccessor:      &OSDiskAccessor{},
		path:                 tempFolder + "/testStorage",
		removeFullBackup:     true,
		fullBackupInProgress: &atomic.Bool{},
	}

	path := tempFolder + "/testStorage/backup/source-ns1/"
	_ = os.MkdirAll(path, 0744)
	_ = backend.writeBackupMetadata(path, model.BackupMetadata{Created: time.UnixMilli(10)})

	to, _ := model.NewTimeBoundsTo(1000)
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
		path:                 tempFolder + "/testStorage",
		removeFullBackup:     false,
		fullBackupInProgress: &atomic.Bool{},
	}

	for _, t := range []int64{10, 20, 30} {
		path := tempFolder + "/testStorage/backup/source-ns1/" + strconv.FormatInt(t, 10) + "/"
		_ = os.MkdirAll(path, 0744)
		_ = backend.writeBackupMetadata(path, model.BackupMetadata{Created: time.UnixMilli(t)})
	}

	bounds, _ := model.NewTimeBoundsTo(25)
	list, _ := backend.FullBackupList(bounds)
	if len(list) != 2 {
		t.Errorf("Expected list size 2, got %v", list)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempFolder)
	})
}
