package service

import (
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// backupTypeMatcher matches a BackupFilter argument by its underlying backup type,
// letting tests distinguish the full-backup lookup from the incremental-backup lookup.
type backupTypeMatcher struct {
	backupType model.BackupType
}

func (m backupTypeMatcher) Matches(x any) bool {
	rf, ok := x.(*RoutineFilter)
	return ok && rf.backupType == m.backupType
}

func (m backupTypeMatcher) String() string {
	return "backupType=" + string(m.backupType)
}

func TestHistoryManager_FindLastRun_NoBackups(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1", Storage: &model.LocalStorage{Path: "/data"}}
	reader := NewMockBackupReader(ctrl)
	reader.EXPECT().
		GetBackups(gomock.Any(), backupTypeMatcher{model.BackupTypeFull}).
		Return(nil, nil)

	hm := NewHistoryManager(reader)
	result, err := hm.FindLastRun(t.Context(), routine)

	require.NoError(t, err)
	assert.True(t, result.NoFullBackup())
}

func TestHistoryManager_FindLastRun_FullOnly(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1", Storage: &model.LocalStorage{Path: "/data"}}
	fullTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	reader := NewMockBackupReader(ctrl)
	reader.EXPECT().
		GetBackups(gomock.Any(), backupTypeMatcher{model.BackupTypeFull}).
		Return([]model.BackupDetails{{BackupMetadata: model.BackupMetadata{Created: fullTime}}}, nil)
	reader.EXPECT().
		GetBackups(gomock.Any(), backupTypeMatcher{model.BackupTypeIncremental}).
		Return(nil, nil)

	hm := NewHistoryManager(reader)
	result, err := hm.FindLastRun(t.Context(), routine)

	require.NoError(t, err)
	require.False(t, result.NoFullBackup())
	assert.Equal(t, fullTime, *result.FullBackupTime())
	assert.Nil(t, result.IncrementalBackupTime())
}

func TestHistoryManager_FindLastRun_FullAndIncremental(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1", Storage: &model.LocalStorage{Path: "/data"}}
	fullTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	incrTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	reader := NewMockBackupReader(ctrl)
	reader.EXPECT().
		GetBackups(gomock.Any(), backupTypeMatcher{model.BackupTypeFull}).
		Return([]model.BackupDetails{{BackupMetadata: model.BackupMetadata{Created: fullTime}}}, nil)
	reader.EXPECT().
		GetBackups(gomock.Any(), backupTypeMatcher{model.BackupTypeIncremental}).
		Return([]model.BackupDetails{{BackupMetadata: model.BackupMetadata{Created: incrTime}}}, nil)

	hm := NewHistoryManager(reader)
	result, err := hm.FindLastRun(t.Context(), routine)

	require.NoError(t, err)
	assert.Equal(t, fullTime, *result.FullBackupTime())
	assert.Equal(t, incrTime, *result.IncrementalBackupTime())
}

func TestHistoryManager_FindLastRun_FullBackupError(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1", Storage: &model.LocalStorage{Path: "/data"}}
	fullErr := errors.New("full backup scan failed")

	reader := NewMockBackupReader(ctrl)
	reader.EXPECT().
		GetBackups(gomock.Any(), backupTypeMatcher{model.BackupTypeFull}).
		Return(nil, fullErr)

	hm := NewHistoryManager(reader)
	_, err := hm.FindLastRun(t.Context(), routine)

	require.Error(t, err)
	require.ErrorIs(t, err, fullErr)
	assert.Contains(t, err.Error(), "read last full backup failed")
}

func TestHistoryManager_FindLastRun_IncrementalBackupError(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1", Storage: &model.LocalStorage{Path: "/data"}}
	fullTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	incrErr := errors.New("incremental backup scan failed")

	reader := NewMockBackupReader(ctrl)
	reader.EXPECT().
		GetBackups(gomock.Any(), backupTypeMatcher{model.BackupTypeFull}).
		Return([]model.BackupDetails{{BackupMetadata: model.BackupMetadata{Created: fullTime}}}, nil)
	reader.EXPECT().
		GetBackups(gomock.Any(), backupTypeMatcher{model.BackupTypeIncremental}).
		Return(nil, incrErr)

	hm := NewHistoryManager(reader)
	_, err := hm.FindLastRun(t.Context(), routine)

	require.Error(t, err)
	require.ErrorIs(t, err, incrErr)
	assert.Contains(t, err.Error(), "read last incremental backup failed")
}
