package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

type ConfigRetriever interface { //nolint:golint
	RetrieveConfiguration(context.Context, string, time.Time) ([]byte, error)
}

// ConfigRetrieverImpl is used to read Aerospike configuration from backup.
type ConfigRetrieverImpl struct {
	backendService BackupBackendService
	config         *model.Config
}

func NewConfigRetriever(backendService BackupBackendService, config *model.Config) *ConfigRetrieverImpl {
	return &ConfigRetrieverImpl{
		backendService: backendService,
		config:         config,
	}
}

// RetrieveConfiguration return backed up Aerospike configuration.
func (cr *ConfigRetrieverImpl) RetrieveConfiguration(ctx context.Context, routine string, toTime time.Time,
) ([]byte, error) {
	backups, err := cr.backendService.GetBackups(ctx, NewFullBackupFilter(routine).WithToTime(toTime).Last())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve backups for routine %s: %w", routine, err)
	}

	if len(backups) == 0 {
		return nil, fmt.Errorf("no full backups found in the specified period of time")
	}

	path := getConfigurationPath(routine, backups[0].Created)

	backupRoutine, _ := cr.config.Routine(routine)
	configBackups, err := storage.ReadFiles(ctx, backupRoutine.Storage, path, configExt)
	if err != nil && !errors.Is(err, storage.ErrEmptyStorage) {
		return nil, err
	}

	if len(configBackups) == 0 {
		return nil, fmt.Errorf("no configuration backups found for %s", path)
	}

	return packageFiles(configBackups)
}

func packageFiles(buffers []*bytes.Buffer) ([]byte, error) {
	// Create a buffer to write our archive to
	buf := new(bytes.Buffer)

	// Create a new zip archive
	w := zip.NewWriter(buf)

	for i, data := range buffers {
		fileName := getConfigFileName(i)

		f, err := w.Create(fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to create entry for filename %s: %w", fileName, err)
		}

		_, err = io.Copy(f, data)
		if err != nil {
			return nil, fmt.Errorf("failed to write buffer %d: %w", i, err)
		}
	}

	err := w.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close the zip writer: %w", err)
	}

	return buf.Bytes(), nil
}
