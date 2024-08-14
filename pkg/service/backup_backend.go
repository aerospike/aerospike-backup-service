package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
	"gopkg.in/yaml.v3"
)

// BackupBackend handles the backup management logic, employing a StorageAccessor
// implementation for I/O operations.
type BackupBackend struct {
	StorageAccessor
	fullBackupsPath        string
	incrementalBackupsPath string
	stateFilePath          string
	removeFullBackup       bool

	// BackupBackend needs to know if full backup is running to filter it out
	fullBackupInProgress *atomic.Bool
	stateFileMutex       sync.RWMutex
}

var _ BackupListReader = (*BackupBackend)(nil)

const metadataFile = "metadata.yaml"

func newBackend(config *model.Config, routineName string) *BackupBackend {
	backupRoutine := config.BackupRoutines[routineName]
	storage := config.Storage[backupRoutine.Storage]
	backupPolicy := config.BackupPolicies[backupRoutine.BackupPolicy]
	removeFullBackup := backupPolicy.RemoveFiles.RemoveFullBackup()
	switch storage.Type {
	case model.Local:
		routinePath := filepath.Join(*storage.Path, routineName)
		return &BackupBackend{
			StorageAccessor:        NewOSDiskAccessor(),
			fullBackupsPath:        filepath.Join(routinePath, model.FullBackupDirectory),
			incrementalBackupsPath: filepath.Join(routinePath, model.IncrementalBackupDirectory),
			stateFilePath:          filepath.Join(routinePath, model.StateFileName),
			removeFullBackup:       removeFullBackup,
			fullBackupInProgress:   &atomic.Bool{},
		}
	case model.S3:
		s3Context := NewS3Context(storage)
		routinePath := filepath.Join(s3Context.path, routineName)
		return &BackupBackend{
			StorageAccessor:        s3Context,
			fullBackupsPath:        filepath.Join(routinePath, model.FullBackupDirectory),
			incrementalBackupsPath: filepath.Join(routinePath, model.IncrementalBackupDirectory),
			stateFilePath:          filepath.Join(routinePath, model.StateFileName),
			removeFullBackup:       removeFullBackup,
			fullBackupInProgress:   &atomic.Bool{},
		}
	default:
		panic(fmt.Sprintf("Unsupported storage type: %v", storage.Type))
	}
}

func (b *BackupBackend) readState() *model.BackupState {
	b.stateFileMutex.RLock()
	defer b.stateFileMutex.RUnlock()
	state := model.NewBackupState()
	err := b.readBackupState(b.stateFilePath, state)
	if err != nil {
		slog.Warn("Failed to read backup state",
			slog.String("path", b.stateFilePath),
			slog.Any("err", err))
	}
	return state
}

func (b *BackupBackend) writeState(state *model.BackupState) error {
	b.stateFileMutex.Lock()
	defer b.stateFileMutex.Unlock()
	return b.writeYaml(b.stateFilePath, state)
}

func (b *BackupBackend) writeBackupMetadata(path string, metadata model.BackupMetadata) error {
	metadataFilePath := filepath.Join(path, metadataFile)
	return b.writeYaml(metadataFilePath, metadata)
}

func (b *BackupBackend) writeYaml(path string, data any) error {
	dataYaml, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	return b.write(path, dataYaml)
}

// FullBackupList returns a list of available full backups.
func (b *BackupBackend) FullBackupList(timebounds *model.TimeBounds) ([]model.BackupDetails, error) {
	slog.Info("Get full backups",
		slog.String("backupFolder", b.fullBackupsPath),
		slog.Any("timebounds", timebounds),
		slog.Bool("removeFullBackup", b.removeFullBackup))

	// when use RemoveFiles.RemoveFullBackup() = true, backup data is located in
	// backupFolder folder itself
	if b.removeFullBackup {
		return b.detailsFromPaths(timebounds, false, b.fullBackupsPath), nil
	}

	return b.fromSubfolders(timebounds, b.fullBackupsPath)
}

// FindLastFullBackup returns last full backup prior to given time.
func (b *BackupBackend) FindLastFullBackup(toTime time.Time) ([]model.BackupDetails, error) {
	fullBackupList, err := b.FullBackupList(model.NewTimeBoundsTo(toTime))
	if err != nil {
		return nil, fmt.Errorf("cannot read full backup list: %w", err)
	}

	fullBackup := latestFullBackupBeforeTime(fullBackupList, toTime) // it's a list of namespaces
	if len(fullBackup) == 0 {
		return nil, fmt.Errorf("%w: %s", errBackupNotFound, toTime)
	}
	return fullBackup, nil
}

// latestFullBackupBeforeTime returns list of backups with same creation time,
// latest before upperBound.
func latestFullBackupBeforeTime(allBackups []model.BackupDetails, upperBound time.Time,
) []model.BackupDetails {
	var result []model.BackupDetails
	var latestTime time.Time
	for i := range allBackups {
		current := &allBackups[i]
		if current.Created.After(upperBound) {
			continue
		}

		if len(result) == 0 || latestTime.Before(current.Created) {
			latestTime = current.Created
			result = []model.BackupDetails{*current}
		} else if current.Created.Equal(latestTime) {
			result = append(result, *current)
		}
	}
	return result
}

// FindIncrementalBackupsForNamespace returns all incremental backups in given range, sorted by time.
func (b *BackupBackend) FindIncrementalBackupsForNamespace(bounds *model.TimeBounds, namespace string,
) ([]model.BackupDetails, error) {
	allIncrementalBackupList, err := b.IncrementalBackupList(bounds)
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

func (b *BackupBackend) detailsFromPaths(timebounds *model.TimeBounds, useCache bool,
	paths ...string) []model.BackupDetails {
	// each path contains a backup of specific time
	backupDetails := make([]model.BackupDetails, 0, len(paths))
	for _, path := range paths {
		namespaces, err := b.lsDir(filepath.Join(path, model.DataDirectory), nil)
		if err != nil {
			slog.Warn("Cannot list backup dir", "path", path, "err", err)
			continue
		}
		for _, namespacePath := range namespaces {
			details, err := b.readBackupDetails(namespacePath, useCache)
			if err != nil {
				slog.Debug("Cannot read backup details", "err", err)
				continue
			}
			if timebounds.Contains(details.Created) {
				backupDetails = append(backupDetails, details)
			}
		}
	}
	return backupDetails
}

func (b *BackupBackend) fromSubfolders(timebounds *model.TimeBounds, backupFolder string,
) ([]model.BackupDetails, error) {
	var after *string
	if timebounds.FromTime != nil {
		after = util.Ptr(formatTime(*timebounds.FromTime))
	}

	subfolders, err := b.lsDir(backupFolder, after)
	if err != nil {
		return nil, err
	}

	return b.detailsFromPaths(timebounds, true, subfolders...), nil
}

// IncrementalBackupList returns a list of available incremental backups.
func (b *BackupBackend) IncrementalBackupList(timebounds *model.TimeBounds,
) ([]model.BackupDetails, error) {
	return b.fromSubfolders(timebounds, b.incrementalBackupsPath)
}

func (b *BackupBackend) FullBackupInProgress() *atomic.Bool {
	return b.fullBackupInProgress
}

func (b *BackupBackend) ReadClusterConfiguration(path string) ([]byte, error) {
	configBackups, err := b.lsFiles(path)
	if err != nil {
		return nil, err
	}
	if len(configBackups) == 0 {
		return nil, fmt.Errorf("no configuration backups found for %s", path)
	}

	return b.packageFiles(configBackups)
}

// PackageFiles creates a zip archive from the given file list and returns it as a byte array.
func (b *BackupBackend) packageFiles(files []string) ([]byte, error) {
	// Create a buffer to write our archive to
	buf := new(bytes.Buffer)

	// Create a new zip archive
	w := zip.NewWriter(buf)

	for _, file := range files {
		data, err := b.read(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file, err)
		}

		f, err := w.Create(filepath.Base(file))
		if err != nil {
			return nil, fmt.Errorf("failed to create entry for filename %s: %w", file, err)
		}

		_, err = f.Write(data)
		if err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", file, err)
		}
	}

	err := w.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close the zip writer: %w", err)
	}

	return buf.Bytes(), nil
}
