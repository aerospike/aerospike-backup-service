package configuration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// fileConfigurationManager implements the Manager interface,
// performing I/O operations on local storage.
type fileConfigurationManager struct {
	sync.Mutex
	FilePath string
}

var _ Manager = (*fileConfigurationManager)(nil)

// newFileConfigurationManager returns a new fileConfigurationManager.
func newFileConfigurationManager(path string) Manager {
	return &fileConfigurationManager{FilePath: path}
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
	defer file.Close()

	return readConfig(file)
}

// Write writes the configuration to the given file path.
func (cm *fileConfigurationManager) Write(ctx context.Context, config *model.Config) error {
	cm.Lock()
	defer cm.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	if cm.FilePath == "" {
		return errors.New("configuration file path is missing")
	}

	file, err := os.OpenFile(cm.FilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open file for writing %q: %w", cm.FilePath, err)
	}
	defer file.Close()

	if err := writeConfig(file, config); err != nil {
		return fmt.Errorf("failed to write configuration to file %q: %w", cm.FilePath, err)
	}

	return nil
}

func (cm *fileConfigurationManager) Update(ctx context.Context, updateFunc func(*model.Config) error) error {
	config, err := cm.Read(ctx)
	if err != nil {
		return fmt.Errorf("failed to read configuration: %w", err)
	}

	if err := updateFunc(config); err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	return cm.Write(ctx, config)
}
