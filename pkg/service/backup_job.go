package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/quartz"
)

// backupJob implements the quartz.Job interface.
type backupJob struct {
	orchestrator *BackupOrchestrator
	routine      *model.BackupRoutine
	jobType      model.BackupJobType
	logger       *slog.Logger
}

var _ quartz.Job = (*backupJob)(nil)

// Execute is called by a Scheduler when the Trigger associated with this job fires.
func (j *backupJob) Execute(ctx context.Context) error {
	jobMetadata, ok := ctx.Value(quartz.JobMetadataContextKey).(quartz.JobMetadata)
	if !ok {
		return errors.New("failed to retrieve job metadata from context. " +
			"Use quartz.WithJobMetadata() option when initializing the scheduler")
	}
	now := time.Unix(0, jobMetadata.RunTime).Truncate(time.Millisecond)

	j.orchestrator.RunBackup(ctx, j.routine, now, j.jobType)

	return nil
}

// Description returns the description of the backup job.
func (j *backupJob) Description() string {
	return fmt.Sprintf("%s %s backup job", j.routine.Name, j.jobType)
}

// newBackupJob creates a new backup job with an immutable routine snapshot.
func newBackupJob(pipeline *BackupOrchestrator, routine *model.BackupRoutine, bt model.BackupJobType) quartz.Job {
	return &backupJob{
		orchestrator: pipeline,
		routine:      routine,
		jobType:      bt,
		logger:       slog.Default().With(attr.Routine(routine.Name), slog.Any("type", bt)),
	}
}
