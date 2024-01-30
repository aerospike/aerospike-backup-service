package service

import (
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aerospike/backup/pkg/model"
)

func TestFullBackupRemoveFiles(t *testing.T) {
	backend := &BackupBackend{
		StorageAccessor:      &OSDiskAccessor{},
		path:                 "./tmp/testStorage",
		removeFiles:          true,
		fullBackupInProgress: &atomic.Bool{},
	}

	os.MkdirAll("./tmp/testStorage/backup/source-ns1/", 0744)
	backend.writeBackupMetadata("./tmp/testStorage/backup/source-ns1/", model.BackupMetadata{Created: time.UnixMilli(10)})

	list, _ := backend.FullBackupList(0, 1000)
	if len(list) != 1 {
		t.Errorf("Expected list size 1, got %v", list)
	}
	t.Cleanup(func() {
		os.RemoveAll("./tmp")
	})
}

func TestFullBackupKeepFiles(t *testing.T) {
	backend := &BackupBackend{
		StorageAccessor:      &OSDiskAccessor{},
		path:                 "./tmp/testStorage",
		removeFiles:          false,
		fullBackupInProgress: &atomic.Bool{},
	}

	for _, t := range []int64{10, 20, 30} {
		path := "./tmp/testStorage/backup/source-ns1/" + strconv.FormatInt(t, 10)
		os.MkdirAll(path, 0744)
		backend.writeBackupMetadata(path, model.BackupMetadata{Created: time.UnixMilli(t)})
	}

	list, _ := backend.FullBackupList(0, 25)
	if len(list) != 2 {
		t.Errorf("Expected list size 2, got %v", list)
	}
	t.Cleanup(func() {
		os.RemoveAll("./tmp")
	})
}
