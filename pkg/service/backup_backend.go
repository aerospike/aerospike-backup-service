package service

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/aerospike/backup/pkg/model"
)

// BackupBackend handles the backup management logic, employing a StorageAccessor implementation
// for I/O operations.
type BackupBackend struct {
	StorageAccessor
	fullBackupsPath        string
	incrementalBackupsPath string
	stateFilePath          string
	removeFullBackup       bool
	fullBackupInProgress   *atomic.Bool // BackupBackend needs to know if full backup is running to filter it out
	stateFileMutex         sync.RWMutex
}

var _ BackupListReader = (*BackupBackend)(nil)

const metadataFile = "metadata.yaml"

func BuildBackupBackends(config *model.Config) map[string]*BackupBackend {
	backends := map[string]*BackupBackend{}
	for routineName := range config.BackupRoutines {
		backends[routineName] = newBackend(config, routineName)
	}
	return backends
}

func newBackend(config *model.Config, routineName string) *BackupBackend {
	backupRoutine := config.BackupRoutines[routineName]
	storage := config.Storage[backupRoutine.Storage]
	backupPolicy := config.BackupPolicies[backupRoutine.BackupPolicy]
	removeFullBackup := backupPolicy.RemoveFiles.RemoveFullBackup()
	switch storage.Type {
	case model.Local:
		path := filepath.Join(*storage.Path, routineName)
		return &BackupBackend{
			StorageAccessor:        NewOSDiskAccessor(),
			fullBackupsPath:        path + "/" + model.FullBackupDirectory,
			incrementalBackupsPath: path + "/" + model.IncrementalBackupDirectory,
			stateFilePath:          path + "/" + model.StateFileName,
			removeFullBackup:       removeFullBackup,
			fullBackupInProgress:   &atomic.Bool{},
		}
	case model.S3:
		s3Context, err := NewS3Context(storage)
		if err != nil {
			panic(err)
		}

		// path is related to storage
		return &BackupBackend{
			StorageAccessor:        s3Context,
			fullBackupsPath:        s3Context.path + "/" + routineName + "/" + model.FullBackupDirectory,
			incrementalBackupsPath: s3Context.path + "/" + routineName + "/" + model.IncrementalBackupDirectory,
			stateFilePath:          s3Context.path + "/" + routineName + "/" + model.StateFileName,
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
		slog.Warn("Failed to read backup state", "path", b.stateFilePath, "err", err)
	}
	return state
}

func (b *BackupBackend) writeState(state *model.BackupState) error {
	b.stateFileMutex.Lock()
	defer b.stateFileMutex.Unlock()
	return b.writeYaml(b.stateFilePath, state)
}

func (b *BackupBackend) writeBackupMetadata(path string, metadata model.BackupMetadata) error {
	return b.writeYaml(filepath.Join(path, metadataFile), metadata)
}

// FullBackupList returns a list of available full backups.
func (b *BackupBackend) FullBackupList(timebounds *model.TimeBounds) ([]model.BackupDetails, error) {
	slog.Info("Get full backups", "backupFolder", b.fullBackupsPath,
		"timebounds", timebounds, "removeFullBackup", b.removeFullBackup)

	// when use RemoveFiles.RemoveFullBackup() = true, backup data is located in backupFolder folder itself
	if b.removeFullBackup {
		return b.detailsFromPaths(timebounds, false, b.fullBackupsPath), nil
	}

	return b.fromSubfolders(timebounds, b.fullBackupsPath)
}

func (b *BackupBackend) detailsFromPaths(timebounds *model.TimeBounds, useCache bool,
	paths ...string) []model.BackupDetails {
	// each path contains a backup of specific time
	backupDetails := []model.BackupDetails{}
	for _, path := range paths {
		namespaces, err := b.lsDir(path)
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
			if timebounds.Contains(details.Created.UnixMilli()) {
				backupDetails = append(backupDetails, details)
			}
		}
	}
	return backupDetails
}

func (b *BackupBackend) fromSubfolders(timebounds *model.TimeBounds,
	backupFolder string) ([]model.BackupDetails, error) {
	subfolders, err := b.lsDir(backupFolder)
	if err != nil {
		return nil, err
	}
	return b.detailsFromPaths(timebounds, true, subfolders...), nil
}

// IncrementalBackupList returns a list of available incremental backups.
func (b *BackupBackend) IncrementalBackupList(timebounds *model.TimeBounds) ([]model.BackupDetails, error) {
	return b.fromSubfolders(timebounds, b.incrementalBackupsPath)
}

func (b *BackupBackend) FullBackupInProgress() *atomic.Bool {
	return b.fullBackupInProgress
}
