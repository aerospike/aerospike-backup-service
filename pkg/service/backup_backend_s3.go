package service

import (
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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
	s3Context := NewS3Context(storage)
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
	lastRun := s.readState().LastFullRun
	slog.Info("get full backups", "backupFolder", backupFolder, "lastRun", lastRun, "from", from, "to", to)
	if s.backupPolicy.RemoveFiles != nil && *s.backupPolicy.RemoveFiles {
		// when use RemoveFiles = true, backup data is located in backupFolder folder itself
		files, _ := s.listFiles(backupFolder)
		if len(files) == 0 {
			return []model.BackupDetails{}, nil
		}
		if s.fullBackupInProgress.Load() {
			return []model.BackupDetails{}, nil
		}
		if lastRun.UnixMilli() < from || lastRun.UnixMilli() >= to {
			return []model.BackupDetails{}, nil
		}
		return []model.BackupDetails{{
			Key:          ptr.String(s3prefix + backupFolder),
			LastModified: &lastRun,
			Size:         ptr.Int64(s.dirSize(backupFolder)),
		}}, nil
	}

	subfolders, err := s.listFolders(backupFolder)
	if err != nil {
		return nil, err
	}
	subfolders, err = filterFolders(subfolders, from, to)
	if err != nil {
		return nil, err
	}
	result := make([]model.BackupDetails, 0, len(subfolders))
	for _, subfolder := range subfolders {
		metadata := s.GetMetadata(subfolder)
		details := model.BackupDetails{
			Key:          ptr.String(s3prefix + "/" + *subfolder.Prefix),
			LastModified: &metadata.Created,
			Size:         ptr.Int64(s.dirSize(*subfolder.Prefix)),
		}
		if !details.LastModified.After(lastRun) {
			result = append(result, details)
		}
	}
	return result, err
}

// filterFolders returns backup folders that were created in the specified time range.
func filterFolders(folders []types.CommonPrefix, from, to int64) ([]types.CommonPrefix, error) {
	result := make([]types.CommonPrefix, 0)
	for _, folder := range folders {
		created, err := strconv.ParseInt(filepath.Base(*folder.Prefix), 10, 64)
		if err != nil {
			return nil, err
		}
		if created >= from && created < to {
			result = append(result, folder)
		}
	}
	return result, nil
}

func (s *BackupBackendS3) dirSize(path string) int64 {
	files, err := s.listFiles(path)
	if err != nil {
		slog.Warn("Failed to list files", "path", path)
		return 0
	}
	var totalSize int64
	for _, file := range files {
		totalSize += *file.Size
	}
	return totalSize
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
	result := make([]model.BackupDetails, 0, len(subfolders))
	for _, subfolder := range subfolders {
		metadata := s.GetMetadata(subfolder)
		details := model.BackupDetails{
			Key:          ptr.String(s3prefix + "/" + *subfolder.Prefix),
			LastModified: &metadata.Created,
			Size:         ptr.Int64(s.dirSize(*subfolder.Prefix)),
		}
		if !details.LastModified.After(lastIncrRun) {
			result = append(result, details)
		}
	}
	return result, err
}

func (s *BackupBackendS3) FullBackupInProgress() *atomic.Bool {
	return s.fullBackupInProgress
}

func (s *BackupBackendS3) writeBackupMetadata(path string, metadata model.BackupMetadata) error {
	s3prefix := "s3://" + s.bucket
	metadataFilePath := strings.TrimPrefix(path, s3prefix) + "/" + metadataFile
	return s.writeFile(metadataFilePath, metadata)
}
