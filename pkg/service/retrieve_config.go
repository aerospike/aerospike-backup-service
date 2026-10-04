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
	"github.com/aerospike/backup-go/io/storage/common"
)

// ConfigRetriever returns the cluster configuration files saved with the last full backup
// at or before a given time, packed into a zip archive.
type ConfigRetriever interface {
	// RetrieveConfiguration returns backed up Aerospike configuration.
	RetrieveConfiguration(context.Context, *model.BackupRoutine, time.Time) ([]byte, error)
}

type configRetriever struct {
	backupReader BackupReader
	pathService  PathService
	operations   storage.Operations
}

var _ ConfigRetriever = (*configRetriever)(nil)

func NewConfigRetriever(
	backupReader BackupReader,
	pathService PathService,
	operations storage.Operations,
) ConfigRetriever {
	return &configRetriever{
		backupReader: backupReader,
		pathService:  pathService,
		operations:   operations,
	}
}

// RetrieveConfiguration return backed up Aerospike configuration.
func (cr *configRetriever) RetrieveConfiguration(
	ctx context.Context, routine *model.BackupRoutine, toTime time.Time,
) ([]byte, error) {
	backups, err := cr.backupReader.GetBackups(ctx, NewFullBackupFilter(routine).WithToTime(toTime).Last())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve backups for routine %s: %w", routine.Name, err)
	}

	if len(backups) == 0 {
		return nil, fmt.Errorf("no full backups found before %v: %w", toTime, model.ErrNotFound)
	}

	path := cr.pathService.GetConfigurationPath(routine.Name, backups[0].Created)
	configBackups, err := cr.operations.ReadFiles(ctx, routine.Storage, path, configExt)
	if err != nil && !errors.Is(err, common.ErrEmptyStorage) {
		return nil, err
	}

	if len(configBackups) == 0 {
		return nil, fmt.Errorf("no configuration backups found for %s: %w", path, model.ErrNotFound)
	}

	return packageFiles(configBackups)
}

func packageFiles(buffers []*bytes.Buffer) ([]byte, error) {
	// Create a buffer to write our archive to
	buf := new(bytes.Buffer)

	// Create a new zip archive
	w := zip.NewWriter(buf)

	for i, data := range buffers {
		fileName := configFileName(i)

		f, err := w.Create(fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to create entry for filename %s: %w", fileName, err)
		}

		_, err = io.Copy(f, data)
		if err != nil {
			return nil, fmt.Errorf("failed to write buffer %d: %w", i, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close the zip writer: %w", err)
	}

	return buf.Bytes(), nil
}
