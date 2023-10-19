package service

import (
	"github.com/aerospike/backup/pkg/model"
)

type S3ConfigurationManager struct {
	configStorage *model.BackupStorage
}

func NewS3ConfigurationManager(configStorage *model.BackupStorage) S3ConfigurationManager {
	return S3ConfigurationManager{configStorage: configStorage}
}

func (s S3ConfigurationManager) ReadConfiguration() (*model.Config, error) {
	panic("implement me")
}

// nolint:revive
func (s S3ConfigurationManager) WriteConfiguration(config *model.Config) error {
	panic("implement me")
}

var _ ConfigurationManager = (*S3ConfigurationManager)(nil)
