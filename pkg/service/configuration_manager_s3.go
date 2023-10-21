package service

import (
	"bytes"
	"context"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gopkg.in/yaml.v3"
	"io"
	"log/slog"
	"net/url"
	"os"
)

type S3ConfigurationManager struct {
	configStorage *model.BackupStorage
	ctx           context.Context
	client        *s3.Client
	bucket        string
	path          string
}

var _ ConfigurationManager = (*S3ConfigurationManager)(nil)

func NewS3ConfigurationManager(configStorage *model.BackupStorage) S3ConfigurationManager {
	// Load the SDK's configuration from environment and shared config, and
	// create the client with this.
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(*configStorage.S3Region),
	)
	if err != nil {
		slog.Error("Failed to load S3 SDK configuration", "err", err)
		os.Exit(1)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(*configStorage.S3EndpointOverride)
		o.UsePathStyle = true
	})

	parsed, err := url.Parse(*configStorage.Path)
	if err != nil {
		slog.Error("Failed to parse S3 storage path", "err", err)
		os.Exit(1)
	}
	return S3ConfigurationManager{ctx: ctx, configStorage: configStorage, path: parsed.Path, bucket: parsed.Host, client: client}
}

func (s S3ConfigurationManager) ReadConfiguration() (*model.Config, error) {
	result, err := s.client.GetObject(s.ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String("s3_config.yml"),
	})
	config := &model.Config{}
	if err != nil {
		slog.Warn("Failed to config state file for backup", "path", s.path)
		return config, nil
	}
	defer result.Body.Close()
	bytes, err := io.ReadAll(result.Body)
	if err != nil {
		slog.Warn("Couldn't read object body of backup state file",
			"path", s.path, "err", err)
	}
	if err = yaml.Unmarshal(bytes, config); err != nil {
		slog.Warn("Failed unmarshal state file for backup",
			"path", s.path, "err", err)
	}
	return config, nil
}

// nolint:revive
func (s S3ConfigurationManager) WriteConfiguration(config *model.Config) error {
	backupState, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(backupState)
	_, err = s.client.PutObject(s.ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.path),
		Body:   reader,
	})
	if err != nil {
		slog.Warn("Couldn't upload state file", "path", s.path,
			"bucket", s.bucket, "err", err)
	}

	return err
}
