package service

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/aerospike/aerospike-backup-service/pkg/dto"
	"gopkg.in/yaml.v3"
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

// ReadConfiguration reads the configuration from the given file path.
func (cm *FileConfigurationManager) ReadConfiguration() (*dto.Config, error) {
	cm.Lock()
	defer cm.Unlock()

	filePath := cm.FilePath
	if filePath == "" {
		return nil, errors.New("configuration file is missing")
	}
	buf, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	config := dto.NewConfigWithDefaultValues()
	err = yaml.Unmarshal(buf, config)
	if err != nil {
		return nil, fmt.Errorf("in file %q: %w", filePath, err)
	}

	err = config.Validate()
	return config, err
}

// WriteConfiguration writes the configuration to the given file path.
func (cm *FileConfigurationManager) WriteConfiguration(config *dto.Config) error {
	cm.Lock()
	defer cm.Unlock()

	filePath := cm.FilePath
	if filePath == "" {
		return errors.New("configuration file path is missing")
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration data: %w", err)
	}

	err = os.WriteFile(filePath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write configuration to file %q: %w", filePath, err)
	}

	return nil
}
