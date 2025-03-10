package service

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullBackupReadFiles(t *testing.T) {
	tempFolder := t.TempDir()
	name := "routine"
	backend := &BackupBackend{
		routine: &model.BackupRoutine{
			Storage: &model.LocalStorage{Path: tempFolder},
		},
		routineName: name,
	}

	for _, t := range []int64{10, 20, 30} {
		path := getBackupPath(name, jobTypeFull, "source-ns1", time.UnixMilli(t))
		_ = os.MkdirAll(path, 0744)
		_ = backend.writeBackupMetadata(context.Background(), path, model.BackupMetadata{Created: time.UnixMilli(t)})
	}

	bounds := model.NewTimeBoundsTo(time.UnixMilli(25))
	list, _ := backend.FullBackupList(context.Background(), bounds)
	require.Equal(t, 2, len(list))
}

func TestPackageFiles(t *testing.T) {
	data := []string{
		"file1 content",
		"file2 content",
		"file3 content",
	}

	var buffers []*bytes.Buffer
	for _, content := range data {
		buffers = append(buffers, bytes.NewBufferString(content))
	}

	backend := &BackupBackend{}
	zipBytes, err := backend.packageFiles(buffers)
	assert.NoError(t, err, "Expected no error from packageFiles")

	reader := bytes.NewReader(zipBytes)
	zipReader, err := zip.NewReader(reader, int64(len(zipBytes)))
	assert.NoError(t, err, "Failed to read zip archive")

	assert.Equal(t, len(buffers), len(zipReader.File))

	for i, file := range zipReader.File {
		expectedFileName := getConfigFileName(i)
		assert.Equal(t, expectedFileName, file.Name)

		rc, err := file.Open()
		assert.NoError(t, err)

		var fileContent bytes.Buffer
		_, err = fileContent.ReadFrom(rc)
		rc.Close()
		assert.NoError(t, err)

		expectedContent := data[i]
		assert.Equal(t, expectedContent, fileContent.String(), "Unexpected file content in zip")
	}
}
