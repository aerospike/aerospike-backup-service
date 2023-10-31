package service

import (
	"fmt"
	"io/fs"
	"os"

	"log/slog"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
	"github.com/aws/smithy-go/ptr"
	"gopkg.in/yaml.v3"
)

// BackupBackendLocal implements the BackupBackend interface by
// saving state to the local file system.
type BackupBackendLocal struct {
	path          string
	stateFilePath string
	backupPolicy  *model.BackupPolicy
}

var _ BackupBackend = (*BackupBackendLocal)(nil)

// NewBackupBackendLocal returns a new BackupBackendLocal instance.
func NewBackupBackendLocal(path string, backupPolicy *model.BackupPolicy) *BackupBackendLocal {
	prepareDirectory(path)
	prepareDirectory(path + "/" + model.IncrementalBackupDirectory)
	prepareDirectory(path + "/" + model.FullBackupDirectory)
	return &BackupBackendLocal{
		path:          path,
		stateFilePath: path + "/" + model.StateFileName,
		backupPolicy:  backupPolicy,
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
			local.stateFilePath, "err", err)
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

	if local.backupPolicy.RemoveFiles != nil && *local.backupPolicy.RemoveFiles {
		// when use RemoveFiles = true, backup data is located in backupFolder folder itself
		if len(entries) > 0 {
			return []model.BackupDetails{{
				Key: ptr.String(backupFolder),
			}}, nil
		}
		return []model.BackupDetails{}, nil
	}

	var backupDetails []model.BackupDetails
	for _, e := range entries {
		if e.IsDir() {
			backupDetails = append(backupDetails, toBackupDetails(e, backupFolder+"/"))
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

	var backupDetails []model.BackupDetails
	for _, e := range entries {
		if !e.IsDir() {
			backupDetails = append(backupDetails, toBackupDetails(e, backupFolder+"/"))
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

// BackupPolicyName returns the name of the defining backup policy.
func (local *BackupBackendLocal) BackupPolicyName() string {
	return *local.backupPolicy.Name
}

func toBackupDetails(e fs.DirEntry, prefix string) model.BackupDetails {
	details := model.BackupDetails{
		Key: util.Ptr(prefix + e.Name()),
	}
	dirInfo, err := e.Info()
	if err == nil {
		details.LastModified = util.Ptr(dirInfo.ModTime())
		details.Size = util.Ptr(dirInfo.Size())
	}
	return details
}
