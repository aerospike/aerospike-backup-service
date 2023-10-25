package service

import (
	"fmt"
	"os"

	"github.com/aerospike/backup/pkg/model"
	"gopkg.in/yaml.v3"
)

type ConfigurationManager interface {
	ReadConfiguration() (*model.Config, error)
	WriteConfiguration(config *model.Config) error
}

func NewConfigurationManager(path string) ConfigurationManager {
	return &FileConfigurationManager{FilePath: path}
}

func ReadConfigStorage(filePath string) (*model.BackupStorage, error) {
	buf, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	configStorage := &model.BackupStorage{}
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
