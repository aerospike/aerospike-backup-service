package service

import (
	"log/slog"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/smithy-go/ptr"
)

// BackupBackendS3 implements the BackupBackend interface by
// saving state to AWS S3.
type BackupBackendS3 struct {
	*S3Context
	stateFilePath string
	backupPolicy  *model.BackupPolicy
}

var _ BackupBackend = (*BackupBackendS3)(nil)

// NewBackupBackendS3 returns a new BackupBackendS3 instance.
func NewBackupBackendS3(storage *model.Storage,
	backupPolicy *model.BackupPolicy) *BackupBackendS3 {
	s3Context := NewS3Context(storage)
	return &BackupBackendS3{
		S3Context:     s3Context,
		stateFilePath: s3Context.Path + "/" + model.StateFileName,
		backupPolicy:  backupPolicy,
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

// FullBackupList returns a list of available full backups.
func (s *BackupBackendS3) FullBackupList() ([]model.BackupDetails, error) {
	backupFolder := s.Path + "/" + model.FullBackupDirectory + "/"
	s3prefix := "s3://" + s.bucket
	if s.backupPolicy.RemoveFiles != nil && *s.backupPolicy.RemoveFiles {
		// when use RemoveFiles = true, backup data is located in backupFolder folder itself
		files, _ := s.listFiles(backupFolder)
		if len(files) > 0 {
			return []model.BackupDetails{{
				Key:          ptr.String(s3prefix + backupFolder),
				LastModified: &s.readState().LastRun,
				Size:         ptr.Int64(s.dirSize(backupFolder)),
			}}, nil
		}
		return []model.BackupDetails{}, nil
	}

	subfolders, err := s.listFolders(backupFolder)
	if err != nil {
		return nil, err
	}
	contents := make([]model.BackupDetails, len(subfolders))
	for i, subfolder := range subfolders {
		details := model.BackupDetails{
			Key:          ptr.String(s3prefix + "/" + *subfolder.Prefix),
			LastModified: s.GetTime(subfolder),
			Size:         ptr.Int64(s.dirSize(*subfolder.Prefix)),
		}
		contents[i] = details
	}
	return contents, err
}

func (s *BackupBackendS3) dirSize(path string) int64 {
	files, err := s.listFiles(path)
	if err != nil {
		slog.Warn("Failed to list files", "path", path)
		return 0
	}
	var totalSize int64
	for _, file := range files {
		totalSize += file.Size
	}
	return totalSize
}

// IncrementalBackupList returns a list of available incremental backups.
func (s *BackupBackendS3) IncrementalBackupList() ([]model.BackupDetails, error) {
	s3prefix := "s3://" + s.bucket
	list, err := s.listFiles(s.Path + "/" + model.IncrementalBackupDirectory)
	if err != nil {
		return nil, err
	}
	contents := make([]model.BackupDetails, len(list))
	for i, object := range list {
		details := model.BackupDetails{
			Key:          ptr.String(s3prefix + "/" + *object.Key),
			LastModified: object.LastModified,
			Size:         &object.Size,
		}
		contents[i] = details
	}
	return contents, err
}
