package service

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"gopkg.in/yaml.v3"
)

// storageOperations abstracts storage I/O for backup metadata.
type storageOperations interface {
	// ReadFileNames lists the names of files in the specified storage matching the filter.
	ReadFileNames(ctx context.Context, storage model.Storage, path string, filterStr string, fromTime *time.Time,
	) ([]string, error)
	// ReadFile reads the content of a file in the specified storage.
	ReadFile(ctx context.Context, storage model.Storage, filePath string) ([]byte, error)
	// WriteMetadataFile writes a metadata file to the specified storage.
	WriteMetadataFile(ctx context.Context, storage model.Storage, fileName string, content []byte) error
	// DeleteFolder deletes a folder and its contents in the specified storage.
	DeleteFolder(ctx context.Context, storage model.Storage, path string) error
}

// BackupReaderWriter defines operations for reading and writing backups metadata.
type BackupReaderWriter interface {
	BackupReader
	BackupWriter
}

// BackupReader defines operations for reading backups metadata.
type BackupReader interface {
	// GetBackups retrieves backup details based on the provided filter.
	GetBackups(ctx context.Context, filter BackupFilter) ([]model.BackupDetails, error)
}

// BackupWriter defines operations for writing backups metadata.
type BackupWriter interface {
	// WriteBackupMetadata stores metadata for a specific backup.
	WriteBackupMetadata(context.Context, *model.BackupRoutine, string, model.BackupMetadata) error

	// Delete removes a specific backup folder.
	Delete(ctx context.Context, routine *model.BackupRoutine, path string) error
}

// BackupBackendServiceImpl default implementation of BackupReaderWriter.
type BackupBackendServiceImpl struct {
	*backupReader
	locks       collections.LockMap // lock per routine
	pathService PathService
	operations  storageOperations
}

var _ BackupReaderWriter = (*BackupBackendServiceImpl)(nil)

func NewBackupBackendService(
	pathService PathService,
	operations storageOperations,
) *BackupBackendServiceImpl {
	return &BackupBackendServiceImpl{
		backupReader: newBackupReader(pathService, operations),
		pathService:  pathService,
		operations:   operations,
	}
}

func (b *BackupBackendServiceImpl) GetBackups(ctx context.Context, filter BackupFilter) ([]model.BackupDetails, error) {
	switch f := filter.(type) {
	case *RoutineFilter:
		lock := b.locks.Get(f.routine.Name)
		lock.RLock()
		defer lock.RUnlock()
		return b.getRoutineBackups(ctx, f)
	case *PathFilter:
		return b.getPathBackups(ctx, f)
	default:
		return nil, fmt.Errorf("unsupported filter type: %T", f)
	}
}

func (b *BackupBackendServiceImpl) WriteBackupMetadata(
	ctx context.Context,
	routine *model.BackupRoutine,
	path string,
	metadata model.BackupMetadata,
) error {
	dataYaml, err := yaml.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metadataFilePath := filepath.Join(path, metadataFile)

	lock := b.locks.Get(routine.Name)
	lock.Lock()
	defer lock.Unlock()

	return b.operations.WriteMetadataFile(ctx, routine.Storage, metadataFilePath, dataYaml)
}

func (b *BackupBackendServiceImpl) Delete(ctx context.Context, routine *model.BackupRoutine, path string) error {
	lock := b.locks.Get(routine.Name)
	lock.Lock()
	defer lock.Unlock()

	err := b.operations.DeleteFolder(ctx, routine.Storage, path)
	if err != nil {
		return fmt.Errorf("failed to delete folder: %w", err)
	}

	slog.Info("Deleted folder", slog.String("path", path), attr.Routine(routine.Name))

	return err
}
