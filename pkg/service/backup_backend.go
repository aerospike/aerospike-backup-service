package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"gopkg.in/yaml.v3"
)

// BackupBackend handles the backup management logic, employing a StorageAccessor
// implementation for I/O operations.
type BackupBackend struct {
	mu          sync.RWMutex
	storage     model.Storage
	routineName string
}

var _ BackupMetadataReaderWriter = (*BackupBackend)(nil)

func newBackend(routineName string, storage model.Storage) *BackupBackend {
	return &BackupBackend{
		storage:     storage,
		routineName: routineName,
	}
}

func (b *BackupBackend) writeBackupMetadata(ctx context.Context, path string, metadata model.BackupMetadata) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	dataYaml, err := yaml.Marshal(metadata)
	if err != nil {
		return err
	}

	metadataFilePath := filepath.Join(path, metadataFile)
	return storage.WriteMetadataFile(ctx, b.storage, metadataFilePath, dataYaml)
}

// IncrementalBackupList returns a list of available incremental backups.
func (b *BackupBackend) IncrementalBackupList(ctx context.Context, timeBounds model.TimeBounds,
) ([]model.BackupDetails, error) {
	return b.readMetadataList(ctx, timeBounds, jobTypeIncremental)
}

func (b *BackupBackend) readMetadataList(
	ctx context.Context, timeBounds model.TimeBounds, backupType jobType,
) ([]model.BackupDetails, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	backupRoot := getBackupRootPath(b.routineName, backupType)
	files, err := storage.ReadFiles(ctx, b.storage, backupRoot, metadataFile, timeBounds.FromTime)
	if err != nil {
		if errors.Is(err, storage.ErrEmptyStorage) {
			return nil, nil
		}
		return nil, fmt.Errorf("read metadata files error: %w", err)
	}

	var backups []model.BackupDetails
	for _, buf := range files {
		metadata, err := model.NewMetadataFromBytes(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("error decoding backup metadata YAML: %w", err)
		}
		if timeBounds.Contains(metadata.Created) {
			backups = append(backups, model.BackupDetails{
				BackupMetadata: *metadata,
				Key:            getKey(b.routineName, backupType, metadata),
				Storage:        b.storage,
			})
		}
	}

	return backups, nil
}

// LastFullBackupTime retrieves the time of the most recent backup full in the specified time range.
func (b *BackupBackend) LastFullBackupTime(ctx context.Context, timeBounds model.TimeBounds) (time.Time, error) {
	return b.lastBackupTime(ctx, timeBounds, jobTypeFull)
}

// LastIncrementalBackupTime retrieves the time of the most recent incremental backup in the specified time range.
func (b *BackupBackend) LastIncrementalBackupTime(ctx context.Context, timeBounds model.TimeBounds) (time.Time, error) {
	return b.lastBackupTime(ctx, timeBounds, jobTypeIncremental)
}

func (b *BackupBackend) lastBackupTime(
	ctx context.Context, timeBounds model.TimeBounds, jobType jobType,
) (time.Time, error) {
	path := getBackupRootPath(b.routineName, jobType)

	// local storage is special case
	local, ok := b.storage.(*model.LocalStorage)
	if ok {
		return lastBackupTimeLocal(ctx, local, path, timeBounds)
	}

	files, err := storage.ReadFileNames(ctx, b.storage, path, metadataFile, timeBounds.FromTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("read metadata files in %s: %w", timeBounds.String(), err)
	}

	if len(files) == 0 {
		return time.Time{}, errBackupNotFound
	}

	// Storage returned all files >= fromTime. We need to find the one with highest timestamp that's still < ToTime.
	// We use the timestamps that are part of file path.
	maxString := "\uffff"
	if timeBounds.ToTime != nil {
		maxString = getTimestampPath(b.routineName, *timeBounds.ToTime, jobType)
	}

	slog.Info("Filtering backups", slog.Any("files", files), slog.String("max", maxString))

	var lastFile string
	for _, file := range files {
		if file > lastFile && file < maxString {
			lastFile = file
		}
	}

	if lastFile == "" {
		return time.Time{}, fmt.Errorf("no backups matching time bounds %s: %w", timeBounds.String(), errBackupNotFound)
	}

	// Only read one (last) file.
	file, err := storage.ReadFile(ctx, b.storage, strings.TrimPrefix(lastFile, b.storage.GetPath()))
	if err != nil {
		return time.Time{}, fmt.Errorf("read metadata file %q error: %w", lastFile, err)
	}

	metadata, err := model.NewMetadataFromBytes(file)
	if err != nil {
		return time.Time{}, fmt.Errorf("error decoding backup metadata YAML: %w", err)
	}

	return metadata.Created, nil
}

// lastBackupTimeLocal finds the latest backup within a specified time range in local storage.
// Local storage does not support listing files in nested folders, but it is fast so we just iterate over all of them.
func lastBackupTimeLocal(
	ctx context.Context, s *model.LocalStorage, path string, timeBounds model.TimeBounds,
) (time.Time, error) {
	files, err := storage.ReadFiles(ctx, s, path, metadataFile, timeBounds.FromTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("read local metadata files in %v: %w", timeBounds, err)
	}

	if len(files) == 0 {
		return time.Time{}, errBackupNotFound
	}

	lastTime := time.Time{}
	for _, buf := range files {
		metadata, err := model.NewMetadataFromBytes(buf.Bytes())
		if err != nil {
			return time.Time{}, fmt.Errorf("error decoding backup metadata YAML: %w", err)
		}
		if metadata.Created.After(lastTime) && timeBounds.Contains(metadata.Created) {
			lastTime = metadata.Created
		}
	}

	return lastTime, nil
}

// FindIncrementalBackupsForNamespace returns all incremental backups in given range, sorted by time.
func (b *BackupBackend) FindIncrementalBackupsForNamespace(
	ctx context.Context, bounds model.TimeBounds, namespace string,
) ([]model.BackupDetails, error) {
	allIncrementalBackupList, err := b.IncrementalBackupList(ctx, bounds)
	if err != nil {
		return nil, err
	}

	var filteredIncrementalBackups []model.BackupDetails
	for _, b := range allIncrementalBackupList {
		if b.Namespace == namespace {
			filteredIncrementalBackups = append(filteredIncrementalBackups, b)
		}
	}
	// Sort in place
	sort.Slice(filteredIncrementalBackups, func(i, j int) bool {
		return filteredIncrementalBackups[i].Created.Before(filteredIncrementalBackups[j].Created)
	})

	return filteredIncrementalBackups, nil
}

// packageFiles creates a zip archive from the given file list and returns it as a byte array.

func (b *BackupBackend) deleteFolder(ctx context.Context, path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return storage.DeleteFolder(ctx, b.storage, path)
}
