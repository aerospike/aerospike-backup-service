package service

import (
	"fmt"
	"github.com/aerospike/backup/pkg/model"
	"log/slog"
	"math"
	"os"
	"sync/atomic"
)

func (b *BackupBackendImpl) readState() *model.BackupState {
	b.stateFileMutex.RLock()
	defer b.stateFileMutex.RUnlock()
	state := model.NewBackupState()
	err := b.readBackupState(b.stateFilePath, state)
	if err != nil {
		slog.Warn("failed to read state " + b.stateFilePath)
	}
	return state
}

func (b *BackupBackendImpl) writeState(state *model.BackupState) error {
	b.stateFileMutex.Lock()
	defer b.stateFileMutex.Unlock()
	return b.writeYaml(b.stateFilePath, state)
}

// FullBackupList returns a list of available full backups.
func (b *BackupBackendImpl) FullBackupList(from, to int64) ([]model.BackupDetails, error) {
	backupFolder := b.path + "/" + model.FullBackupDirectory + "/"
	slog.Info("Get full backups", "backupFolder", backupFolder, "from", from, "to", to)

	// when use RemoveFiles = true, backup data is located in backupFolder folder itself
	if b.backupPolicy.RemoveFiles != nil && *b.backupPolicy.RemoveFiles {
		return b.detailsFromPaths(from, to, removeLeadingSlash(backupFolder)), nil
	}

	return b.fromSubfolders(from, to, backupFolder)
}

func (b *BackupBackendImpl) detailsFromPaths(from, to int64, paths ...string) []model.BackupDetails {
	slog.Info("detailsFromPaths", "from", from, "to", to, "paths", paths)
	backupDetails := []model.BackupDetails{}
	for _, path := range paths {
		details, err := b.readBackupDetails(path)
		if err != nil {
			slog.Warn("cannot read details", "basePath", path, "err", err)
			continue
		}
		if details.Created.UnixMilli() >= from &&
			details.Created.UnixMilli() < to {
			backupDetails = append(backupDetails, details)
		} else {
			slog.Info("Skipped " + details.String())
		}
	}
	return backupDetails
}

func (b *BackupBackendImpl) fromSubfolders(from, to int64, backupFolder string) ([]model.BackupDetails, error) {
	subfolders, err := b.lsDir(backupFolder)
	if err != nil {
		return nil, err
	}

	return b.detailsFromPaths(from, to, subfolders...), nil
}

// IncrementalBackupList returns a list of available incremental backups.
func (b *BackupBackendImpl) IncrementalBackupList() ([]model.BackupDetails, error) {
	backupFolder := b.path + "/" + model.IncrementalBackupDirectory
	return b.fromSubfolders(0, math.MaxInt64, backupFolder)
}

func (b *BackupBackendImpl) FullBackupInProgress() *atomic.Bool {
	return b.fullBackupInProgress
}

func (b *BackupBackendImpl) writeBackupMetadata(path string, metadata model.BackupMetadata) error {
	return b.writeYaml(path+"/"+metadataFile, metadata)
}

func (b *BackupBackendImpl) CleanDir(name string) error {
	path := fmt.Sprintf("%s/%s/", b.path, name)
	dir, err := os.ReadDir(path)
	if err != nil {
		slog.Warn("Failed to read directory", "basePath", path, "err", err)
	}
	for _, e := range dir {
		filePath := path + "/" + e.Name()
		if err = b.DeleteFolder(filePath); err != nil {
			return err
		}
	}
	return nil
}
