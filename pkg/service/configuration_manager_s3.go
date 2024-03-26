package service

import (
	"fmt"

	"github.com/aerospike/backup/pkg/model"
)

// FileConfigurationManager implements the ConfigurationManager interface,
// performing I/O operations on AWS S3.
type S3ConfigurationManager struct {
	*S3Context
}

var _ ConfigurationManager = (*S3ConfigurationManager)(nil)

// NewS3ConfigurationManager returns a new S3ConfigurationManager.
func NewS3ConfigurationManager(configStorage *model.Storage) (ConfigurationManager, error) {
	s3Context, err := NewS3Context(configStorage)
	if err != nil {
		return nil, err
	}
	err = configStorage.Validate()
	if err != nil {
		return nil, err
	}
	return &S3ConfigurationManager{
		s3Context,
	}, nil
}

// ReadConfiguration reads and returns the configuration from S3.
func (s *S3ConfigurationManager) ReadConfiguration() (*model.Config, error) {
	config := model.NewConfigWithDefaultValues()
	err := s.readFile(s.path, config)
	if err != nil {
		return nil, fmt.Errorf("cannot read file %s: %v", s.path, err)
	}
	err = config.Validate()
	if err != nil {
		return nil, err
	}
	return config, nil
}

// WriteConfiguration writes the configuration to S3.
func (s *S3ConfigurationManager) WriteConfiguration(config *model.Config) error {
	return s.writeYaml(s.path, config)
}
