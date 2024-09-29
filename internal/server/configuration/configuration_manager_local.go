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
func (cm *FileConfigurationManager) ReadConfiguration(_ context.Context) (io.ReadCloser, error) {
	cm.Lock()
	defer cm.Unlock()

	filePath := cm.FilePath
	if filePath == "" {
		return nil, errors.New("configuration file is missing")
	}

	return os.Open(filePath)
}

// WriteConfiguration writes the configuration to the given file path.
func (cm *FileConfigurationManager) WriteConfiguration(_ context.Context, config *model.Config) error {
	cm.Lock()
	defer cm.Unlock()

	filePath := cm.FilePath
	if filePath == "" {
		return errors.New("configuration file path is missing")
	}

	configDto := dto.NewConfigFromModel(config)
	data, err := dto.Serialize(configDto, dto.YAML)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration data: %w", err)
	}

	err = os.WriteFile(filePath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write configuration to file %q: %w", filePath, err)
	}

	return nil
}
