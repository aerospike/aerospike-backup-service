package service

import (
	"github.com/aerospike/backup/pkg/model"
)

// FileConfigurationManager implements the ConfigurationManager interface,
// performing I/O operations on AWS S3.
type S3ConfigurationManager struct {
	*S3Context
}

var _ ConfigurationManager = (*S3ConfigurationManager)(nil)

// NewS3ConfigurationManager returns a new S3ConfigurationManager.
func NewS3ConfigurationManager(configStorage *model.BackupStorage) ConfigurationManager {
	s3Context := NewS3Context(configStorage)
	return &S3ConfigurationManager{s3Context}
}

// ReadConfiguration reads and returns the configuration from S3.
func (s *S3ConfigurationManager) ReadConfiguration() (*model.Config, error) {
	config := model.NewConfigWithDefaultValues()
	s.readFile(s.Path, config)
	return config, nil
}

// WriteConfiguration writes the configuration to S3.
func (s *S3ConfigurationManager) WriteConfiguration(config *model.Config) error {
	return s.writeFile(s.Path, config)
}
