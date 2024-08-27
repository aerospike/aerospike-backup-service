package service

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/internal/server/dto"
	"github.com/aerospike/aerospike-backup-service/pkg/model"
)

// S3ConfigurationManager implements the ConfigurationManager interface,
// performing I/O operations on AWS S3.
type S3ConfigurationManager struct {
	*S3Context
}

var _ ConfigurationManager = (*S3ConfigurationManager)(nil)

// S3ManagerBuilder defines the interface for building S3ConfigurationManager.
type S3ManagerBuilder interface {
	// NewS3ConfigurationManager returns a new S3ConfigurationManager.
	NewS3ConfigurationManager(configStorage *model.Storage) (ConfigurationManager, error)
}

type S3ManagerBuilderImpl struct{}

var _ S3ManagerBuilder = &S3ManagerBuilderImpl{}

func (builder S3ManagerBuilderImpl) NewS3ConfigurationManager(configStorage *model.Storage,
) (ConfigurationManager, error) {
	return &S3ConfigurationManager{
		NewS3Context(configStorage),
	}, nil
}

// ReadConfiguration reads and returns the configuration from S3.
func (s *S3ConfigurationManager) ReadConfiguration() (*model.Config, error) {
	config := dto.NewConfigWithDefaultValues()
	err := s.readFile(s.path, config)
	if err != nil {
		return nil, fmt.Errorf("cannot read file %s: %w", s.path, err)
	}

	err = config.Validate()
	if err != nil {
		return nil, err
	}

	return config.ToModel(), nil
}

// WriteConfiguration writes the configuration to S3.
func (s *S3ConfigurationManager) WriteConfiguration(config *model.Config) error {
	return s.writeYaml(s.path, config)
}
