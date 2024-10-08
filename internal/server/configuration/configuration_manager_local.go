package configuration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// FileConfigurationManager implements the Manager interface,
// performing I/O operations on local storage.
type FileConfigurationManager struct {
	sync.Mutex
	FilePath string
}

var _ Manager = (*FileConfigurationManager)(nil)

// NewFileConfigurationManager returns a new FileConfigurationManager.
func NewFileConfigurationManager(path string) Manager {
	return &FileConfigurationManager{FilePath: path}
}

// ReadConfiguration returns a reader for the configuration file.
func (cm *FileConfigurationManager) ReadConfiguration(ctx context.Context) (io.ReadCloser, error) {
	cm.Lock()
	defer cm.Unlock()

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

	return file, nil
}

// WriteConfiguration writes the configuration to the given file path.
func (cm *FileConfigurationManager) WriteConfiguration(ctx context.Context, config *model.Config) error {
	cm.Lock()
	defer cm.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	if cm.FilePath == "" {
		return errors.New("configuration file path is missing")
	}

	configDto := dto.NewConfigFromModel(config)
	data, err := dto.Serialize(configDto, dto.YAML)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration data: %w", err)
	}

	err = os.WriteFile(cm.FilePath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write configuration to file %q: %w", cm.FilePath, err)
	}

	return nil
}
