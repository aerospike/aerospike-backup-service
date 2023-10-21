package service

import (
	"log/slog"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// BackupBackendS3 implements the BackupBackend interface by
// saving state to AWS S3.
type BackupBackendS3 struct {
	*S3Context
	stateFilePath    string
	backupPolicyName string
}

var _ BackupBackend = (*BackupBackendS3)(nil)

// NewBackupBackendS3 returns a new BackupBackendS3 instance.
func NewBackupBackendS3(storage *model.BackupStorage, backupPolicyName string) *BackupBackendS3 {
	s3Context := NewS3Context(storage)
	return &BackupBackendS3{
		S3Context:        s3Context,
		stateFilePath:    s3Context.Path + "/" + stateFileName,
		backupPolicyName: backupPolicyName,
	}
}

func (s *BackupBackendS3) readState() *model.BackupState {
	state := model.NewBackupState()
	s.readFile(s.stateFilePath, state)
	return state
}

func (s *BackupBackendS3) writeState(state *model.BackupState) error {
	return s.writeFile(s.stateFilePath, state)
}

func (s *BackupBackendS3) FullBackupList() ([]model.BackupDetails, error) {
	result, err := s.List(s.Path + "/backup")
	if err != nil {
		slog.Warn("Couldn't list backups in bucket", "path", s.Path, "err", err)
		return nil, err
	}
	list := result.CommonPrefixes
	contents := make([]model.BackupDetails, 0, len(list))
	for i, prefix := range list {
		details := model.BackupDetails{
			Key: prefix.Prefix,
		}
		contents[i] = details
	}
	return contents, err
}

func (s *BackupBackendS3) IncrementalBackupList() ([]model.BackupDetails, error) {
	result, err := s.List(s.Path + "/" + incremenalBackupDirectory)

	if err != nil {
		slog.Warn("Couldn't list incremental backups", "path", s.Path, "err", err)
		return nil, err
	}

	list := result.Contents
	contents := make([]model.BackupDetails, 0, len(list))
	for i, object := range list {
		details := model.BackupDetails{
			Key:          object.Key,
			LastModified: object.LastModified,
			Size:         &object.Size,
		}
		contents[i] = details
	}
	return contents, err
}

func (s *BackupBackendS3) CleanDir(name string) {
	path := s.Path + "/" + name
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
