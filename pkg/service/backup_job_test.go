package service

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/quartz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBackupOrchestrator struct {
	called     bool
	gotRoutine *model.BackupRoutine
	gotNow     time.Time
	gotType    model.BackupType
}

func (f *fakeBackupOrchestrator) Backup(
	_ context.Context, routine *model.BackupRoutine, now time.Time, backupType model.BackupType,
) {
	f.called = true
	f.gotRoutine = routine
	f.gotNow = now
	f.gotType = backupType
}

func TestBackupJob_Execute_Success(t *testing.T) {
	t.Parallel()

	orchestrator := &fakeBackupOrchestrator{}
	routine := &model.BackupRoutine{Name: "daily"}
	job := newBackupJob(orchestrator, routine, model.BackupTypeFull)

	runTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()
	ctx := context.WithValue(t.Context(), quartz.JobMetadataContextKey, quartz.JobMetadata{RunTime: runTime})

	err := job.Execute(ctx)
	require.NoError(t, err)
	assert.True(t, orchestrator.called)
	assert.Equal(t, routine, orchestrator.gotRoutine)
	assert.Equal(t, model.BackupTypeFull, orchestrator.gotType)
	assert.Equal(t, time.Unix(0, runTime).Truncate(time.Millisecond), orchestrator.gotNow)
}

func TestBackupJob_Execute_MissingMetadata(t *testing.T) {
	t.Parallel()

	orchestrator := &fakeBackupOrchestrator{}
	routine := &model.BackupRoutine{Name: "daily"}
	job := newBackupJob(orchestrator, routine, model.BackupTypeFull)

	err := job.Execute(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve job metadata")
	assert.False(t, orchestrator.called)
}

func TestBackupJob_Description(t *testing.T) {
	t.Parallel()

	orchestrator := &fakeBackupOrchestrator{}
	routine := &model.BackupRoutine{Name: "daily"}
	job := newBackupJob(orchestrator, routine, model.BackupTypeIncremental)

	assert.Equal(t, "daily incremental backup job", job.Description())
}
