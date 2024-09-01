package service

import (
	"errors"
	"fmt"
	"github.com/aerospike/aerospike-backup-service/pkg/model"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aerospike/aerospike-backup-service/pkg/util"
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
	logger := slog.Default().With(slog.String("path", filepath))
	bytes, err := os.ReadFile(filepath)
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) &&
			strings.Contains(pathErr.Error(), "no such file or directory") {
			logger.Debug("State file does not exist for backup",
				slog.Any("err", err))
			return nil
		}
		logger.Warn("Failed to read state file for backup",
			slog.Any("err", err))
		return fmt.Errorf("failed read backup state: %w", err)
	}
	if err = yaml.Unmarshal(bytes, state); err != nil {
		logger.Warn("Failed unmarshal state file for backup",
			slog.String("content", string(bytes)),
			slog.Any("err", err))
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

func (o *OSDiskAccessor) Read(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (o *OSDiskAccessor) write(filePath string, data []byte) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0600)
}

func (o *OSDiskAccessor) lsDir(path string, after *string) ([]string, error) {
	var result []string

	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory
		if p == path {
			return nil
		}

		// We're only interested in directories
		if !d.IsDir() {
			return nil
		}

		// Get the relative path
		relPath, err := filepath.Rel(path, p)
		if err != nil {
			return fmt.Errorf("error getting relative path: %w", err)
		}

		// If 'after' is set, skip directories that come before to it lexicographically
		if after != nil && relPath < *after {
			return nil
		}

		result = append(result, p)
		return filepath.SkipDir // Don't descend into this directory
	})

	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	sort.Strings(result) // Ensure consistent ordering
	return result, nil
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

func (o *OSDiskAccessor) DeleteFolder(pathToDelete string) error {
	slog.Debug("Delete folder", "path", pathToDelete)
	err := os.RemoveAll(pathToDelete)
	if err != nil {
		return err
	}

	parentDir := filepath.Dir(pathToDelete)
	lsDir, err := o.lsDir(parentDir, nil)
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

func validatePathContainsBackup(path string) (uint64, error) {
	_, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	accessor := OSDiskAccessor{}
	var count uint64
	if metadata, err := accessor.readBackupDetails(path, false); err == nil {
		count = metadata.RecordCount
	}

	absFiles, err := filepath.Glob(filepath.Join(path, "*.asb"))
	if err != nil {
		return 0, err
	}
	if len(absFiles) == 0 {
		return 0, fmt.Errorf("no backup files found in %s", path)
	}

	return count, nil
}
