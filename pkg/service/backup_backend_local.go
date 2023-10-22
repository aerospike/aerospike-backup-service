package service

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"

	"log/slog"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
)

// BackupBackendLocal implements the BackupBackend interface by
// saving state to the local file system.
type BackupBackendLocal struct {
	path             string
	stateFilePath    string
	backupPolicyName string
}

var _ BackupBackend = (*BackupBackendLocal)(nil)

// NewBackupBackendLocal returns a new BackupBackendLocal instance.
func NewBackupBackendLocal(path, backupPolicyName string) *BackupBackendLocal {
	prepareDirectory(path)
	incrDirectoryPath := path + "/" + model.IncrementalBackupDirectory
	prepareDirectory(incrDirectoryPath)
	return &BackupBackendLocal{
		path:             path,
		stateFilePath:    path + "/" + model.StateFileName,
		backupPolicyName: backupPolicyName,
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
	if err = json.Unmarshal(bytes, state); err != nil {
		slog.Warn("Failed unmarshal state file for backup", "path",
			local.stateFilePath, "err", err)
	}
	return state
}

func (local *BackupBackendLocal) writeState(state *model.BackupState) error {
	backupState, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(local.stateFilePath, backupState, 0644)
}

func (local *BackupBackendLocal) FullBackupList() ([]model.BackupDetails, error) {
	entries, err := os.ReadDir(local.path)
	if err != nil {
		return nil, err
	}

	var backupDetails []model.BackupDetails
	for _, e := range entries {
		if e.IsDir() {
			backupDetails = append(backupDetails, toBackupDetails(e))
		}
	}
	return backupDetails, nil
}

func (local *BackupBackendLocal) IncrementalBackupList() ([]model.BackupDetails, error) {
	entries, err := os.ReadDir(local.path + "/" + model.IncrementalBackupDirectory)
	if err != nil {
		return nil, err
	}

	var backupDetails []model.BackupDetails
	for _, e := range entries {
		if !e.IsDir() {
			backupDetails = append(backupDetails, toBackupDetails(e))
		}
	}
	return backupDetails, nil
}

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

func (local *BackupBackendLocal) BackupPolicyName() string {
	return local.backupPolicyName
}

func toBackupDetails(e fs.DirEntry) model.BackupDetails {
	details := model.BackupDetails{
		Key: util.Ptr(e.Name()),
	}
	dirInfo, err := e.Info()
	if err == nil {
		details.LastModified = util.Ptr(dirInfo.ModTime())
		details.Size = util.Ptr(dirInfo.Size())
	}
	return details
}
