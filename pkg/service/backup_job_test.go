package service

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/quartz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBackupJob_Execute_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := &model.BackupRoutine{Name: "daily"}
	runTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()
	expectedNow := time.Unix(0, runTime).Truncate(time.Millisecond)

	orchestrator := NewMockBackupOrchestrator(ctrl)
	orchestrator.EXPECT().
		Backup(gomock.Any(), routine, expectedNow, model.BackupTypeFull)

	job := newBackupJob(orchestrator, routine, model.BackupTypeFull)

	ctx := context.WithValue(t.Context(), quartz.JobMetadataContextKey, quartz.JobMetadata{RunTime: runTime})

	err := job.Execute(ctx)
	require.NoError(t, err)
}

func TestBackupJob_Execute_MissingMetadata(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orchestrator := NewMockBackupOrchestrator(ctrl)
	routine := &model.BackupRoutine{Name: "daily"}
	job := newBackupJob(orchestrator, routine, model.BackupTypeFull)

	err := job.Execute(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve job metadata")
}

func TestBackupJob_Description(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orchestrator := NewMockBackupOrchestrator(ctrl)
	routine := &model.BackupRoutine{Name: "daily"}
	job := newBackupJob(orchestrator, routine, model.BackupTypeIncremental)

	assert.Equal(t, "daily incremental backup job", job.Description())
}
