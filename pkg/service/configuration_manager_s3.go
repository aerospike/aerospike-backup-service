package service

import (
	"github.com/aerospike/backup/pkg/model"
)

type S3ConfigurationManager struct {
	*S3Context
}

var _ ConfigurationManager = (*S3ConfigurationManager)(nil)

func NewS3ConfigurationManager(configStorage *model.BackupStorage) S3ConfigurationManager {
	s3Context := NewS3Context(configStorage)
	return S3ConfigurationManager{s3Context}
}

func (s S3ConfigurationManager) ReadConfiguration() (*model.Config, error) {
	config := model.NewConfigWithDefaultValues()
	s.readFile(s.Path, config)
	return config, nil
}

func (s S3ConfigurationManager) WriteConfiguration(config *model.Config) error {
	return s.writeFile(s.Path, config)
}
