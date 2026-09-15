package configuration

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
)

// configFileMode is the mode Write leaves the configuration file in. The file holds literal
// cluster passwords, cloud keys and encryption key secrets, so only its owner may read it.
const configFileMode = 0o600

// fileConfigurationManager keeps the service configuration in a local file.
type fileConfigurationManager struct {
	sync.Mutex
	FilePath    string
	nsValidator aerospike.NamespaceValidator
}

var _ Manager = (*fileConfigurationManager)(nil)

// newFileConfigurationManager returns a new fileConfigurationManager.
func newFileConfigurationManager(path string, nsValidator aerospike.NamespaceValidator) Manager {
	return &fileConfigurationManager{
		FilePath:    path,
		nsValidator: nsValidator,
	}
}

// ReadConfiguration returns a reader for the configuration file.
func (cm *fileConfigurationManager) Read(ctx context.Context) (*model.Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if cm.FilePath == "" {
		return nil, errors.New("configuration file path is missing")
	}

	file, err := os.Open(cm.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", cm.FilePath, err)
	}
	defer func() { _ = file.Close() }()

	return readConfig(ctx, file, cm.nsValidator)
}

// Write writes the configuration to the given file path.
// The new configuration is rendered into a sibling temporary file and renamed over the
// target, so an interrupted write leaves the previous configuration in place rather than an
// empty or half-written one. The file ends up readable by its owner only.
func (cm *fileConfigurationManager) Write(ctx context.Context, config *model.Config) error {
	cm.Lock()
	defer cm.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	if cm.FilePath == "" {
		return errors.New("configuration file path is missing")
	}

	tmpPath, err := cm.writeTempFile(config)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpPath) }() // a no-op once the rename below has succeeded

	if err := os.Rename(tmpPath, cm.FilePath); err != nil {
		// The target cannot be replaced as a whole: it is a bind-mounted file, or it lives in
		// a directory this process may not write. Writing through it is what the service did
		// before, and the only way left to persist the change.
		slog.Warn("Cannot replace the configuration file atomically, writing in place",
			slog.String("path", cm.FilePath), attr.Error(err))

		return cm.writeInPlace(config)
	}

	syncDir(filepath.Dir(cm.FilePath))

	return nil
}

// writeTempFile renders the configuration into a sibling of the target file and returns its
// path. The contents are flushed to disk before the caller publishes them with a rename.
func (cm *fileConfigurationManager) writeTempFile(config *model.Config) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(cm.FilePath), filepath.Base(cm.FilePath)+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("failed to create a temporary file next to %q: %w", cm.FilePath, err)
	}

	if err := writeAndSync(file, config); err != nil {
		return "", errors.Join(
			fmt.Errorf("failed to write configuration to file %q: %w", file.Name(), err),
			file.Close(),
			os.Remove(file.Name()))
	}

	if err := file.Close(); err != nil {
		return "", errors.Join(
			fmt.Errorf("failed to close file %q: %w", file.Name(), err),
			os.Remove(file.Name()))
	}

	// os.CreateTemp already creates the file with configFileMode; setting it keeps the
	// guarantee in this file rather than in the standard library's documentation.
	if err := os.Chmod(file.Name(), configFileMode); err != nil {
		return "", errors.Join(
			fmt.Errorf("failed to restrict the mode of file %q: %w", file.Name(), err),
			os.Remove(file.Name()))
	}

	return file.Name(), nil
}

// writeInPlace truncates the target and writes through it. It is the fallback for a target
// that cannot be replaced by a rename, and carries that path's torn-file risk.
func (cm *fileConfigurationManager) writeInPlace(config *model.Config) error {
	file, err := os.OpenFile(cm.FilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, configFileMode)
	if err != nil {
		return fmt.Errorf("failed to open file for writing %q: %w", cm.FilePath, err)
	}

	if err := writeAndSync(file, config); err != nil {
		return errors.Join(
			fmt.Errorf("failed to write configuration to file %q: %w", cm.FilePath, err),
			file.Close())
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close file %q: %w", cm.FilePath, err)
	}

	// The mode of an existing file survives O_CREATE, and a bind-mounted file may not be
	// ours to chmod: tighten it where we can and say so where we cannot.
	if err := os.Chmod(cm.FilePath, configFileMode); err != nil {
		slog.Warn("Cannot restrict the mode of the configuration file, it may be readable by other local users",
			slog.String("path", cm.FilePath), attr.Error(err))
	}

	return nil
}

func writeAndSync(file *os.File, config *model.Config) error {
	if err := writeConfig(file, config); err != nil {
		return err
	}

	return file.Sync()
}

// syncDir flushes a directory entry so that a rename into it survives a crash. Failing here
// means the new configuration is in place but not yet durable, which is not worth reporting
// the write as failed.
func syncDir(path string) {
	dir, err := os.Open(path)
	if err != nil {
		slog.Warn("Cannot open the configuration directory to flush it",
			slog.String("path", path), attr.Error(err))
		return
	}
	defer func() { _ = dir.Close() }()

	if err := dir.Sync(); err != nil {
		slog.Warn("Cannot flush the configuration directory",
			slog.String("path", path), attr.Error(err))
	}
}
