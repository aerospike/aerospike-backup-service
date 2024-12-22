package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestFullBackupReadFiles(t *testing.T) {
	tempFolder := t.TempDir()
	name := "routine"
	backend := &BackupBackend{
		storage:     &model.LocalStorage{Path: tempFolder},
		routineName: name,
	}

	for _, t := range []int64{10, 20, 30} {
		path := fmt.Sprintf("routine/backup/%d/data/source-ns1/", t)
		_ = os.MkdirAll(path, 0744)
		_ = backend.writeBackupMetadata(context.Background(), path, model.BackupMetadata{Created: time.UnixMilli(t)})
	}

	bounds := model.NewTimeBoundsTo(time.UnixMilli(25))
	list, _ := backend.FullBackupList(context.Background(), bounds)
	require.Equal(t, 2, len(list))
}
