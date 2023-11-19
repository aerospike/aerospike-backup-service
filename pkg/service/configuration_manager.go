package service

import (
	"fmt"
	"os"

	"github.com/aerospike/backup/pkg/model"
	"gopkg.in/yaml.v3"
)

// ConfigurationManager represents a configuration file handler.
type ConfigurationManager interface {
	ReadConfiguration() (*model.Config, error)
	WriteConfiguration(config *model.Config) error
}

// ReadConfigStorage reads and returns remote configuration details.
func ReadConfigStorage(filePath string) (*model.Storage, error) {
	buf, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	configStorage := &model.Storage{}
	err = yaml.Unmarshal(buf, configStorage)
	if err != nil {
		return nil, fmt.Errorf("in file %q: %w", filePath, err)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return configStorage, nil
}
