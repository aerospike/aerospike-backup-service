package configuration

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v2/internal/server/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// FileConfigurationManager implements the ConfigurationManager interface,
// performing I/O operations on local storage.
type FileConfigurationManager struct {
	sync.Mutex
	FilePath string
}

var _ ConfigurationManager = (*FileConfigurationManager)(nil)

// NewFileConfigurationManager returns a new FileConfigurationManager.
func NewFileConfigurationManager(path string) ConfigurationManager {
	return &FileConfigurationManager{FilePath: path}
}

// ReadConfiguration returns a reader for the configuration file.
func (cm *FileConfigurationManager) ReadConfiguration() (io.ReadCloser, error) {
	cm.Lock()
	defer cm.Unlock()

	filePath := cm.FilePath
	if filePath == "" {
		return nil, errors.New("configuration file is missing")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// WriteConfiguration writes the configuration to the given file path.
func (cm *FileConfigurationManager) WriteConfiguration(config *model.Config) error {
	cm.Lock()
	defer cm.Unlock()

	filePath := cm.FilePath
	if filePath == "" {
		return errors.New("configuration file path is missing")
	}

	configDto := dto.NewConfigFromModel(config)
	data, err := configDto.Serialize(dto.YAML)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration data: %w", err)
	}

	err = os.WriteFile(filePath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write configuration to file %q: %w", filePath, err)
	}

	return nil
}
