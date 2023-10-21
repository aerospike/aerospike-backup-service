package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/url"
	"os"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// BackupBackendS3 implements the BackupBackend interface by
// saving state to AWS S3.
type BackupBackendS3 struct {
	ctx              context.Context
	client           *s3.Client
	bucket           string
	path             string
	stateFilePath    string
	backupPolicyName string
}

var _ BackupBackend = (*BackupBackendS3)(nil)

// NewBackupBackendS3 returns a new BackupBackendS3 instance.
func NewBackupBackendS3(storage *model.BackupStorage, backupPolicyName string) *BackupBackendS3 {
	// Load the SDK's configuration from environment and shared config, and
	// create the client with this.
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(*storage.S3Region),
	)

	if err != nil {
		slog.Error("Failed to load S3 SDK configuration", "err", err)
		os.Exit(1)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(*storage.S3EndpointOverride)
		o.UsePathStyle = true
	})

	parsed, err := url.Parse(*storage.Path)
	if err != nil {
		slog.Error("Failed to parse S3 storage path", "err", err)
		os.Exit(1)
	}

	return &BackupBackendS3{
		ctx:              ctx,
		client:           client,
		bucket:           parsed.Host,
		path:             parsed.Path,
		stateFilePath:    parsed.Path + "/" + stateFileName,
		backupPolicyName: backupPolicyName,
	}
}

func (s *BackupBackendS3) readState() *model.BackupState {
	result, err := s.client.GetObject(s.ctx, &s3.GetObjectInput{
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
		slog.Warn("Couldn't read object body of backup state file",
			"path", s.stateFilePath, "err", err)
	}
	if err = json.Unmarshal(bytes, state); err != nil {
		slog.Warn("Failed unmarshal state file for backup",
			"path", s.stateFilePath, "err", err)
	}
	return state
}

func (s *BackupBackendS3) writeState(state *model.BackupState) error {
	backupState, err := json.Marshal(state)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(backupState)
	_, err = s.client.PutObject(s.ctx, &s3.PutObjectInput{
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

func (s *BackupBackendS3) FullBackupList() ([]model.BackupDetails, error) {
	result, err := s.client.ListObjectsV2(s.ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(s.path + "/"),
		Delimiter: aws.String("/"),
	})
	var contents []model.BackupDetails
	if err != nil {
		slog.Warn("Couldn't list backups in bucket", "path", s.path, "err", err)
	} else {
		for _, prefix := range result.CommonPrefixes {
			details := model.BackupDetails{
				Key: prefix.Prefix,
			}
			contents = append(contents, details)
		}
	}
	return contents, err
}

func (s *BackupBackendS3) IncrementalBackupList() ([]model.BackupDetails, error) {
	result, err := s.client.ListObjectsV2(s.ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(s.path + "/" + incremenalBackupDirectory + "/"),
		Delimiter: aws.String(""),
	})
	var contents []model.BackupDetails
	if err != nil {
		slog.Warn("Couldn't list incremental backups", "path", s.path, "err", err)
	} else {
		for _, object := range result.Contents {
			details := model.BackupDetails{
				Key:          object.Key,
				LastModified: object.LastModified,
				Size:         &object.Size,
			}
			contents = append(contents, details)
		}
	}
	return contents, err
}

func (s *BackupBackendS3) CleanDir(name string) {
	path := s.path + "/" + name
	result, err := s.client.ListObjectsV2(s.ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(path),
		Delimiter: aws.String(""),
	})
	if err != nil {
		slog.Warn("Couldn't list files in directory", "path", path, "err", err)
	} else {
		for _, file := range result.Contents {
			_, err := s.client.DeleteObject(s.ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(s.bucket),
				Key:    file.Key,
			})
			if err != nil {
				slog.Debug("Couldn't delete file", "path", *file.Key, "err", err)
			}
		}
	}
}

func (s *BackupBackendS3) BackupPolicyName() string {
	return s.backupPolicyName
}
