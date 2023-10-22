package service

import (
	"github.com/aerospike/backup/pkg/model"
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
		stateFilePath:    s3Context.Path + "/" + model.StateFileName,
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
	list, err := s.ListFolders(s.Path + "/" + model.FullBackupDirectory + "/")
	if err != nil {
		return nil, err
	}
	contents := make([]model.BackupDetails, len(list))
	for i, object := range list {
		details := model.BackupDetails{
			Key: object.Prefix,
		}
		contents[i] = details
	}
	return contents, err
}

func (s *BackupBackendS3) IncrementalBackupList() ([]model.BackupDetails, error) {
	list, err := s.ListFiles(s.Path + "/" + model.IncrementalBackupDirectory)
	if err != nil {
		return nil, err
	}
	contents := make([]model.BackupDetails, len(list))
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

func (s *BackupBackendS3) BackupPolicyName() string {
	return s.backupPolicyName
}
