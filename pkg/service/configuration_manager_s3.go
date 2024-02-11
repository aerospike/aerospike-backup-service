package service

import (
	"github.com/aerospike/backup/pkg/model"
)

// FileConfigurationManager implements the ConfigurationManager interface,
// performing I/O operations on AWS S3.
type S3ConfigurationManager struct {
	*S3Context
	FilePath string
}

var _ ConfigurationManager = (*S3ConfigurationManager)(nil)

// NewS3ConfigurationManager returns a new S3ConfigurationManager.
func NewS3ConfigurationManager(configStorage *model.Storage) ConfigurationManager {
	s3Context, _ := NewS3Context(configStorage)
	return &S3ConfigurationManager{
		S3Context: s3Context,
		FilePath:  *configStorage.Path,
	}
}

// ReadConfiguration reads and returns the configuration from S3.
func (s *S3ConfigurationManager) ReadConfiguration() (*model.Config, error) {
	config := model.NewConfigWithDefaultValues()
	_ = s.readFile(s.FilePath, config)
	err := config.Validate()
	return config, err
}

// WriteConfiguration writes the configuration to S3.
func (s *S3ConfigurationManager) WriteConfiguration(config *model.Config) error {
	return s.writeYaml(s.FilePath, config)
}
