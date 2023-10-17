package service

import "github.com/aerospike/backup/pkg/model"

type ConfigurationManager interface {
	ReadConfiguration() (*model.Config, error)
	WriteConfiguration(config *model.Config) error
}

func NewConfigurationManager(path string) ConfigurationManager {
	return &FileConfigurationManager{FilePath: path}
}
