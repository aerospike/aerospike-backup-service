package service

import "github.com/aerospike/backup/pkg/model"

type S3ConfigurationManager struct {
	Path string
}

func (s S3ConfigurationManager) ReadConfiguration() (*model.Config, error) {
	panic("implement me")
}

func (s S3ConfigurationManager) WriteConfiguration(config *model.Config) error {
	panic("implement me")
}

var _ ConfigurationManager = (*S3ConfigurationManager)(nil)
