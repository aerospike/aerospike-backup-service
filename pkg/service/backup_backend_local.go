package service

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"

	"log/slog"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
	"github.com/aws/smithy-go/ptr"
	"gopkg.in/yaml.v3"
)

// BackupBackendLocal implements the BackupBackend interface by
// saving state to the local file system.
type BackupBackendLocal struct {
	path                 string
	stateFilePath        string
	backupPolicy         *model.BackupPolicy
	fullBackupInProgress *atomic.Bool // BackupBackend needs to know if full backup is running to filter it out
}

var _ BackupBackend = (*BackupBackendLocal)(nil)

// NewBackupBackendLocal returns a new BackupBackendLocal instance.
func NewBackupBackendLocal(storage *model.Storage, backupPolicy *model.BackupPolicy,
	fullBackupInProgress *atomic.Bool) *BackupBackendLocal {
	path := *storage.Path
	prepareDirectory(path)
	prepareDirectory(path + "/" + model.IncrementalBackupDirectory)
	prepareDirectory(path + "/" + model.FullBackupDirectory)
	return &BackupBackendLocal{
		path:                 path,
		stateFilePath:        path + "/" + model.StateFileName,
		backupPolicy:         backupPolicy,
		fullBackupInProgress: fullBackupInProgress,
	}
}

func prepareDirectory(path string) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err = os.Mkdir(path, 0744); err != nil {
			slog.Warn("Error creating backup directory", "path", path, "err", err)
		}
	}
	if err = os.Chmod(path, 0744); err != nil {
		slog.Warn("Failed to Chmod backup directory", "path", path, "err", err)
	}
}

func (local *BackupBackendLocal) readState() *model.BackupState {
	bytes, err := os.ReadFile(local.stateFilePath)
	state := model.NewBackupState()
	if err != nil {
		slog.Warn("Failed to read state file for backup", "err", err)
		return state
	}
	if err = yaml.Unmarshal(bytes, state); err != nil {
		slog.Warn("Failed unmarshal state file for backup", "path",
			local.stateFilePath, "err", err, "content", string(bytes))

	}
	return state
}

func (local *BackupBackendLocal) writeState(state *model.BackupState) error {
	backupState, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(local.stateFilePath, backupState, 0644)
}

// FullBackupList returns a list of available full backups.
func (local *BackupBackendLocal) FullBackupList() ([]model.BackupDetails, error) {
	backupFolder := local.path + "/" + model.FullBackupDirectory
	entries, err := os.ReadDir(backupFolder)
	if err != nil {
		return nil, err
	}

	lastRun := local.readState().LastRun
	if local.backupPolicy.RemoveFiles != nil && *local.backupPolicy.RemoveFiles {
		// when use RemoveFiles = true, backup data is located in backupFolder folder itself
		if len(entries) == 0 {
			return []model.BackupDetails{}, nil
		}
		if local.fullBackupInProgress.Load() {
			return []model.BackupDetails{}, nil
		}
		return []model.BackupDetails{{
			Key:          ptr.String(backupFolder),
			LastModified: &lastRun,
		}}, nil
	}

	backupDetails := make([]model.BackupDetails, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			details := toBackupDetails(e, backupFolder)
			if details.LastModified.Before(lastRun) {
				backupDetails = append(backupDetails, details)
			}
		}
	}
	return backupDetails, nil
}

// IncrementalBackupList returns a list of available incremental backups.
func (local *BackupBackendLocal) IncrementalBackupList() ([]model.BackupDetails, error) {
	backupFolder := local.path + "/" + model.IncrementalBackupDirectory
	entries, err := os.ReadDir(backupFolder)
	if err != nil {
		return nil, err
	}
	lastIncrRun := local.readState().LastIncrRun
	backupDetails := make([]model.BackupDetails, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			details := toBackupDetails(e, backupFolder)
			if details.LastModified.Before(lastIncrRun) {
				backupDetails = append(backupDetails, details)
			}
		}
	}
	return backupDetails, nil
}

// CleanDir cleans the directory with the given name.
func (local *BackupBackendLocal) CleanDir(name string) {
	path := fmt.Sprintf("%s/%s/", local.path, name)
	dir, err := os.ReadDir(path)
	if err != nil {
		slog.Warn("Failed to read directory", "path", path, "err", err)
	}
	for _, e := range dir {
		if !e.IsDir() {
			filePath := path + "/" + e.Name()
			if err = os.Remove(filePath); err != nil {
				slog.Debug("Failed to delete file", "path", filePath, "err", err)
			}
		}
	}
}

func (local *BackupBackendLocal) DeleteFile(path string) error {
	err := os.Remove(path)
	if err != nil {
		return fmt.Errorf("failed to delete file: %v", err)
	}
	return nil
}

func toBackupDetails(e fs.DirEntry, prefix string) model.BackupDetails {
	details := model.BackupDetails{
		Key: util.Ptr(filepath.Join(prefix, e.Name())),
	}
	dirInfo, err := e.Info()
	if err == nil {
		details.LastModified = util.Ptr(dirInfo.ModTime())
		details.Size = util.Ptr(dirEntrySize(prefix, e, dirInfo))
	}
	return details
}

func dirEntrySize(path string, e fs.DirEntry, info fs.FileInfo) int64 {
	if e.IsDir() {
		var totalSize int64
		path = filepath.Join(path, e.Name())
		entries, err := os.ReadDir(path)
		if err == nil {
			for _, dirEntry := range entries {
				dirInfo, err := dirEntry.Info()
				if err == nil {
					if dirEntry.IsDir() {
						totalSize += dirEntrySize(path, dirEntry, dirInfo)
					} else {
						totalSize += dirInfo.Size()
					}
				}
			}
		}
		return totalSize
	}
	return info.Size()
}
