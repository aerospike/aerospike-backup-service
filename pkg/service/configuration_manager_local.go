package service

import (
	"errors"
	"fmt"
	"os"

	"github.com/aerospike/backup/pkg/model"
	"gopkg.in/yaml.v3"
)

type FileConfigurationManager struct {
	FilePath string
}

var _ ConfigurationManager = (*FileConfigurationManager)(nil)

// ReadConfiguration reads the configuration from the given file path.
func (cm *FileConfigurationManager) ReadConfiguration() (*model.Config, error) {
	filePath := cm.FilePath
	if filePath == "" {
		return nil, errors.New("configuration file is missing")
	}
	buf, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	config := &model.Config{}
	err = yaml.Unmarshal(buf, config)
	if err != nil {
		return nil, fmt.Errorf("in file %q: %w", filePath, err)
	}

	return config, err
}

// WriteConfiguration writes the configuration to the given file path.
func (cm *FileConfigurationManager) WriteConfiguration(config *model.Config) error {
	filePath := cm.FilePath
	if filePath == "" {
		return errors.New("configuration file path is missing")
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration data: %w", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write configuration to file %q: %w", filePath, err)
	}

	return nil
}
