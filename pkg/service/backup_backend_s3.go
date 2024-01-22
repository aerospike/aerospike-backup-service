package service

import (
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/smithy-go/ptr"
)

// BackupBackendS3 implements the BackupBackend interface by
// saving state to AWS S3.
type BackupBackendS3 struct {
	*S3Context
	stateFilePath        string
	backupPolicy         *model.BackupPolicy
	fullBackupInProgress *atomic.Bool // BackupBackend needs to know if full backup is running to filter it out
	stateFileMutex       sync.RWMutex
}

var _ BackupBackend = (*BackupBackendS3)(nil)

// NewBackupBackendS3 returns a new BackupBackendS3 instance.
func NewBackupBackendS3(storage *model.Storage, backupPolicy *model.BackupPolicy) *BackupBackendS3 {
	s3Context, err := NewS3Context(storage)
	if err != nil {
		panic(err)
	}
	return &BackupBackendS3{
		S3Context:            s3Context,
		stateFilePath:        s3Context.Path + "/" + model.StateFileName,
		backupPolicy:         backupPolicy,
		fullBackupInProgress: &atomic.Bool{},
	}
}

func (s *BackupBackendS3) readState() *model.BackupState {
	s.stateFileMutex.RLock()
	defer s.stateFileMutex.RUnlock()

	state := model.NewBackupState()
	s.readFile(s.stateFilePath, state)
	return state
}

func (s *BackupBackendS3) writeState(state *model.BackupState) error {
	s.stateFileMutex.Lock()
	defer s.stateFileMutex.Unlock()

	return s.writeFile(s.stateFilePath, state)
}

// FullBackupList returns a list of available full backups.
func (s *BackupBackendS3) FullBackupList(from, to int64) ([]model.BackupDetails, error) {
	backupFolder := s.Path + "/" + model.FullBackupDirectory + "/"
	s3prefix := "s3://" + s.bucket
	slog.Info("get full backups", "backupFolder", backupFolder, "from", from, "to", to)
	if s.backupPolicy.RemoveFiles != nil && *s.backupPolicy.RemoveFiles {
		// when use RemoveFiles = true, backup data is located in backupFolder folder itself
		if s.fullBackupInProgress.Load() {
			return []model.BackupDetails{}, nil
		}
		metadata, err := s.readMetadata(backupFolder)
		if err != nil {
			return []model.BackupDetails{}, err
		}
		if metadata.Created.UnixMilli() < from ||
			metadata.Created.UnixMilli() > to {
			return []model.BackupDetails{}, nil
		}
		return []model.BackupDetails{{
			BackupMetadata: *metadata,
			Key:            ptr.String(s3prefix + backupFolder),
		}}, nil
	}

	subfolders, err := s.listFolders(backupFolder)
	if err != nil {
		return nil, err
	}
	backupDetails := make([]model.BackupDetails, 0, len(subfolders))
	for _, subfolder := range subfolders {
		metadata, err := s.GetMetadata(subfolder)
		if err != nil {
			continue
		}
		details := model.BackupDetails{
			BackupMetadata: *metadata,
			Key:            ptr.String(s3prefix + "/" + *subfolder.Prefix),
		}
		if details.Created.UnixMilli() >= from &&
			details.Created.UnixMilli() < to {
			backupDetails = append(backupDetails, details)
		}
	}
	return backupDetails, nil
}

// IncrementalBackupList returns a list of available incremental backups.
func (s *BackupBackendS3) IncrementalBackupList() ([]model.BackupDetails, error) {
	s3prefix := "s3://" + s.bucket
	backupFolder := s.Path + "/" + model.IncrementalBackupDirectory + "/"
	subfolders, err := s.listFolders(backupFolder)
	if err != nil {
		return nil, err
	}
	lastIncrRun := s.readState().LastIncrRun
	backupDetails := make([]model.BackupDetails, 0, len(subfolders))
	for _, subfolder := range subfolders {
		metadata, err := s.GetMetadata(subfolder)
		if err != nil {
			continue
		}
		details := model.BackupDetails{
			BackupMetadata: *metadata,
			Key:            ptr.String(s3prefix + "/" + *subfolder.Prefix),
		}
		if !details.Created.After(lastIncrRun) {
			backupDetails = append(backupDetails, details)
		}
	}
	return backupDetails, nil
}

func (s *BackupBackendS3) FullBackupInProgress() *atomic.Bool {
	return s.fullBackupInProgress
}

func (s *BackupBackendS3) writeBackupMetadata(path string, metadata model.BackupMetadata) error {
	s3prefix := "s3://" + s.bucket
	metadataFilePath := strings.TrimPrefix(path, s3prefix) + "/" + metadataFile
	return s.writeFile(metadataFilePath, metadata)
}
