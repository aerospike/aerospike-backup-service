package service

import (
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
func NewBackupBackendS3(storage *model.BackupStorage,
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
	if s.backupPolicy.RemoveFiles != nil && *s.backupPolicy.RemoveFiles {
		files, _ := s.listFiles(backupFolder)
		if len(files) > 0 {
			return []model.BackupDetails{{
				Key: ptr.String(backupFolder),
			}}, nil
		}
		return []model.BackupDetails{}, nil
	}

	list, err := s.listFolders(backupFolder)
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

// IncrementalBackupList returns a list of available incremental backups.
func (s *BackupBackendS3) IncrementalBackupList() ([]model.BackupDetails, error) {
	list, err := s.listFiles(s.Path + "/" + model.IncrementalBackupDirectory)
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

// BackupPolicyName returns the name of the defining backup policy.
func (s *BackupBackendS3) BackupPolicyName() string {
	return *s.backupPolicy.Name
}
