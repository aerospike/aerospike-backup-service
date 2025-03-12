package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
	routineName string
	routine     *model.BackupRoutine
}

var _ BackupMetadataReaderWriter = (*BackupBackend)(nil)

func newBackend(routineName string, routine *model.BackupRoutine) *BackupBackend {
	return &BackupBackend{
		routineName: routineName,
		routine:     routine,
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
	return storage.WriteMetadataFile(ctx, b.routine.Storage, metadataFilePath, dataYaml)
}

// FullBackupList returns a list of available full backups.
func (b *BackupBackend) FullBackupList(ctx context.Context, timeBounds model.TimeBounds,
) ([]model.BackupDetails, error) {
	return b.readMetadataList(ctx, timeBounds, jobTypeFull)
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
	files, err := storage.ReadFiles(ctx, b.routine.Storage, backupRoot, metadataFile, timeBounds.FromTime)
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
				Storage:        b.routine.Storage,
			})
		}
	}

	return backups, nil
}

func (b *BackupBackend) LastBackupTime(ctx context.Context, timeBounds model.TimeBounds, jobType jobType) (time.Time, error) {
	path := getBackupRootPath(b.routineName, jobType)

	// local storage is special case
	local, ok := b.routine.Storage.(*model.LocalStorage)
	if ok {
		return LastBackupTimeLocal(ctx, local, path, timeBounds)
	}

	files, err := storage.ReadFileNames(ctx, b.routine.Storage, path, metadataFile, timeBounds.FromTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("read metadata files in %v: %w", timeBounds, err)
	}

	if len(files) == 0 {
		return time.Time{}, errBackupNotFound
	}

	maxString := "\uffff"
	if timeBounds.ToTime != nil {
		maxString = getTimestampPath(b.routineName, *timeBounds.ToTime, jobType)
	}

	var lastFile string
	for _, file := range files {
		if file > lastFile && file <= maxString {
			lastFile = file
		}
	}

	if lastFile == "" {
		return time.Time{}, fmt.Errorf("no backups matching time bounds %v: %w", timeBounds, errBackupNotFound)
	}

	file, err := storage.ReadFile(ctx, b.routine.Storage, strings.TrimPrefix(lastFile, b.routine.Storage.GetPath()))
	if err != nil {
		return time.Time{}, fmt.Errorf("read metadata file %q error: %w", lastFile, err)
	}

	metadata, err := model.NewMetadataFromBytes(file)
	if err != nil {
		return time.Time{}, fmt.Errorf("error decoding backup metadata YAML: %w", err)
	}

	return metadata.Created, nil
}

// LastBackupTimeLocal finds the latest backup within a specified time range in local storage.
// Local storage does not support listing files in nested folders, but it is fast so we can just iterate over all of them.
func LastBackupTimeLocal(ctx context.Context, s *model.LocalStorage, path string, timeBounds model.TimeBounds) (time.Time, error) {
	files, err := storage.ReadFiles(ctx, s, path, metadataFile, timeBounds.FromTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("read metadata files in %v: %w", timeBounds, err)
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

func (b *BackupBackend) ReadClusterConfiguration(ctx context.Context, path string) ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	configBackups, err := storage.ReadFiles(ctx, b.routine.Storage, path, configExt, nil)
	if err != nil && !errors.Is(err, storage.ErrEmptyStorage) {
		return nil, err
	}
	if len(configBackups) == 0 {
		return nil, fmt.Errorf("no configuration backups found for %s", path)
	}

	return b.packageFiles(configBackups)
}

// packageFiles creates a zip archive from the given file list and returns it as a byte array.
func (b *BackupBackend) packageFiles(buffers []*bytes.Buffer) ([]byte, error) {
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

func (b *BackupBackend) deleteFolder(ctx context.Context, path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return storage.DeleteFolder(ctx, b.routine.Storage, path)
}
