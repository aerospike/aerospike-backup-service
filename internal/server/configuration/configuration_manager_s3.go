package configuration

import (
	"io"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service"
)

// S3ConfigurationManager implements the ConfigurationManager interface,
// performing I/O operations on AWS S3.
type S3ConfigurationManager struct {
	*service.S3Context
}

var _ ConfigurationManager = (*S3ConfigurationManager)(nil)

func newS3ConfigurationManager(configStorage *model.Storage) (ConfigurationManager, error) {
	return &S3ConfigurationManager{
		service.NewS3Context(configStorage),
	}, nil
}

// ReadConfiguration reads and returns the configuration from S3.
func (s *S3ConfigurationManager) ReadConfiguration() (io.ReadCloser, error) {
	return s.Read(s.Path)
}

// WriteConfiguration writes the configuration to S3.
func (s *S3ConfigurationManager) WriteConfiguration(config *model.Config) error {
	return s.WriteYaml(s.Path, config)
}
