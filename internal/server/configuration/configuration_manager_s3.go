package configuration

import (
	"io"

	"github.com/aerospike/aerospike-backup-service/v2/internal/server/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service"
)

// S3ConfigurationManager implements the Manager interface,
// performing I/O operations on AWS S3.
type S3ConfigurationManager struct {
	*service.S3Context
}

var _ Manager = (*S3ConfigurationManager)(nil)

func newS3ConfigurationManager(configStorage *model.Storage) (Manager, error) {
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
	configDto := dto.NewConfigFromModel(config)
	data, err := dto.Serialize(configDto, dto.YAML)
	if err != nil {
		return err
	}

	return s.Write(s.Path, data)
}
