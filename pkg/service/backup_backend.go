package service

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/aerospike/backup/pkg/model"
)

// BackupBackend handles the backup management logic, employing a StorageAccessor implementation
// for I/O operations.
type BackupBackend struct {
	StorageAccessor
	path                 string
	stateFilePath        string
	removeFullBackup     bool
	fullBackupInProgress *atomic.Bool // BackupBackend needs to know if full backup is running to filter it out
	stateFileMutex       sync.RWMutex
}

var _ BackupListReader = (*BackupBackend)(nil)

const metadataFile = "metadata.yaml"

func BuildBackupBackends(config *model.Config) map[string]*BackupBackend {
	backends := map[string]*BackupBackend{}
	for routineName := range config.BackupRoutines {
		backend := newBackend(config, routineName)
		slog.Debug("New backup backend created", "backend", backend)
		backends[routineName] = backend
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
		path := *storage.Path
		return &BackupBackend{
			StorageAccessor:      NewOSDiskAccessor(),
			path:                 path,
			stateFilePath:        path + "/" + model.StateFileName,
			removeFullBackup:     removeFullBackup,
			fullBackupInProgress: &atomic.Bool{},
		}
	case model.S3:
		s3Context, err := NewS3Context(storage)
		if err != nil {
			panic(err)
		}

		return &BackupBackend{
			StorageAccessor:      s3Context,
			path:                 s3Context.path,
			stateFilePath:        s3Context.path + "/" + model.StateFileName,
			removeFullBackup:     removeFullBackup,
			fullBackupInProgress: &atomic.Bool{},
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

// FullBackupList returns a list of available full backups.
func (b *BackupBackend) FullBackupList(timebounds *model.TimeBounds) ([]model.BackupDetails, error) {
	backupFolder := b.path + "/" + model.FullBackupDirectory + "/"
	slog.Info("Get full backups", "backupFolder", backupFolder, "timebounds", timebounds)

	// when use RemoveFiles.RemoveFullBackup() = true, backup data is located in backupFolder folder itself
	if b.removeFullBackup {
		return b.detailsFromPaths(timebounds, false, removeLeadingSlash(backupFolder)), nil
	}

	return b.fromSubfolders(timebounds, backupFolder)
}

func (b *BackupBackend) detailsFromPaths(timebounds *model.TimeBounds, useCache bool, paths ...string) []model.BackupDetails {
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

func (b *BackupBackend) fromSubfolders(timebounds *model.TimeBounds, backupFolder string) ([]model.BackupDetails, error) {
	subfolders, err := b.lsDir(backupFolder)
	if err != nil {
		return nil, err
	}
	if len(subfolders) == 0 {
		slog.Info("No subfolders found in backup folder", "backupFolder", backupFolder)
		return []model.BackupDetails{}, nil
	}
	return b.detailsFromPaths(timebounds, true, subfolders...), nil
}

// IncrementalBackupList returns a list of available incremental backups.
func (b *BackupBackend) IncrementalBackupList(timebounds *model.TimeBounds) ([]model.BackupDetails, error) {
	backupFolder := b.path + "/" + model.IncrementalBackupDirectory
	return b.fromSubfolders(timebounds, backupFolder)
}

func (b *BackupBackend) FullBackupInProgress() *atomic.Bool {
	return b.fullBackupInProgress
}

func (b *BackupBackend) writeBackupMetadata(path string, metadata model.BackupMetadata) error {
	return b.writeYaml(path+metadataFile, metadata)
}
