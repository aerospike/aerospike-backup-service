package service

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
	"gopkg.in/yaml.v3"
)

// OSDiskAccessor is responsible for IO operation on local disk.
type OSDiskAccessor struct {
}

var _ StorageAccessor = (*OSDiskAccessor)(nil)

// NewOSDiskAccessor returns a new OSDiskAccessor.
func NewOSDiskAccessor() *OSDiskAccessor {
	return &OSDiskAccessor{}
}

func (o *OSDiskAccessor) readBackupState(filepath string, state *model.BackupState) error {
	bytes, err := os.ReadFile(filepath)
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) &&
			strings.Contains(pathErr.Error(), "no such file or directory") {
			slog.Debug("State file does not exist for backup", "path", filepath,
				"err", err)
			return nil
		}
		slog.Warn("Failed to read state file for backup", "path", filepath,
			"err", err)
		return err
	}
	if err = yaml.Unmarshal(bytes, state); err != nil {
		slog.Warn("Failed unmarshal state file for backup", "path", filepath,
			"err", err, "content", string(bytes))
	}
	return nil
}

func (o *OSDiskAccessor) readBackupDetails(path string, _ bool) (model.BackupDetails, error) {
	filePath := filepath.Join(path, metadataFile)
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return model.BackupDetails{}, err
	}

	metadata := &model.BackupMetadata{}
	if err = yaml.Unmarshal(bytes, metadata); err != nil {
		slog.Warn("Failed unmarshal metadata file", "path", filePath, "err", err,
			"content", string(bytes))
		return model.BackupDetails{}, err
	}
	return model.BackupDetails{
		BackupMetadata: *metadata,
		Key:            util.Ptr(path),
	}, nil
}

func (o *OSDiskAccessor) read(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

func (o *OSDiskAccessor) write(filePath string, data []byte) error {
	return os.WriteFile(filePath, data, 0644)
}

func (o *OSDiskAccessor) lsDir(path string) ([]string, error) {
	content, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var onlyDirs []string
	for _, c := range content {
		if c.IsDir() {
			fullPath := filepath.Join(path, c.Name())
			onlyDirs = append(onlyDirs, fullPath)
		}
	}
	return onlyDirs, nil
}

func (o *OSDiskAccessor) lsFiles(path string) ([]string, error) {
	content, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var files []string
	for _, c := range content {
		if !c.IsDir() {
			fullPath := filepath.Join(path, c.Name())
			files = append(files, fullPath)
		}
	}
	return files, nil
}

func (o *OSDiskAccessor) CreateFolder(path string) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(path, 0744); err != nil {
			slog.Warn("Error creating backup directory", "path", path, "err", err)
		}
	}
	if err = os.Chmod(path, 0744); err != nil {
		slog.Warn("Failed to Chmod backup directory", "path", path, "err", err)
	}
}

func (o *OSDiskAccessor) DeleteFolder(pathToDelete string) error {
	slog.Debug("Delete folder", "path", pathToDelete)
	err := os.RemoveAll(pathToDelete)
	if err != nil {
		return err
	}

	parentDir := filepath.Dir(pathToDelete)
	lsDir, err := o.lsDir(parentDir)
	if err != nil {
		return err
	}

	if len(lsDir) == 0 {
		err := os.Remove(parentDir)
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *OSDiskAccessor) wrapWithPrefix(path string) *string {
	return &path
}

func validatePathContainsBackup(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return err
	}

	absFiles, err := filepath.Glob(filepath.Join(path, "*.asb"))
	if err != nil {
		return err
	}
	if len(absFiles) == 0 {
		return fmt.Errorf("no backup files found in %s", path)
	}
	return nil
}
