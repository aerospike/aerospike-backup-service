package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// BackupBackendS3 implements the BackupBackend interface by
// saving state to AWS S3.
type BackupBackendS3 struct {
	client        *s3.Client
	bucket        string
	path          string
	stateFilePath string
}

var _ BackupBackend = (*BackupBackendS3)(nil)

// NewBackupBackendS3 returns a new BackupBackendS3 instance.
func NewBackupBackendS3(storage *model.BackupStorage) *BackupBackendS3 {
	// Load the SDK's configuration from environment and shared config, and
	// create the client with this.
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile(*storage.S3Profile))
	if err != nil {
		slog.Error("Failed to load S3 SDK configuration", "err", err)
		os.Exit(1)
	}
	cfg.Region = *storage.S3Region

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(*storage.S3EndpointOverride)
		o.UsePathStyle = true
	})

	parsed, err := url.Parse(*storage.Path)
	if err != nil {
		slog.Error("Failed to parse S3 storage path", "err", err)
		os.Exit(1)
	}
	keyPath := strings.TrimLeft(parsed.Path, "/")
	return &BackupBackendS3{
		client:        client,
		bucket:        parsed.Host,
		path:          keyPath,
		stateFilePath: keyPath + "/" + stateFileName,
	}
}

func (s *BackupBackendS3) readState() *model.BackupState {
	result, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.stateFilePath),
	})
	state := model.NewBackupState()
	if err != nil {
		slog.Warn("Failed to read state file for backup", "path", s.stateFilePath)
		return state
	}
	defer result.Body.Close()
	bytes, err := io.ReadAll(result.Body)
	if err != nil {
		slog.Warn("Couldn't read object body of backup state file", "path", s.stateFilePath,
			"err", err)
	}
	if err = json.Unmarshal(bytes, state); err != nil {
		slog.Warn("Failed unmarshal state file for backup", "path", s.stateFilePath)
	}
	return state
}

func (s *BackupBackendS3) writeState(state *model.BackupState) error {
	backupState, err := json.Marshal(state)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(backupState)
	_, err = s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.stateFilePath),
		Body:   reader,
	})
	if err != nil {
		slog.Warn("Couldn't upload state file", "path", s.stateFilePath,
			"bucket", s.bucket, "err", err)
	}

	return err
}

func (s *BackupBackendS3) backupList() ([]string, error) {
	result, err := s.client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(s.path),
	})
	var contents []string
	if err != nil {
		slog.Warn("Couldn't list objects in bucket", "path", s.stateFilePath)
	} else {
		for _, metadata := range result.Contents {
			contents = append(contents, *metadata.Key)
		}
	}
	return contents, err
}
