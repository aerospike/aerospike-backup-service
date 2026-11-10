package scanbackup

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
)

// RestoreExecutor restores one scan backup with backup-go.
type RestoreExecutor interface {
	// Run starts the restore of the backup the request names and returns the handler to follow it.
	Run(ctx context.Context, client aerospike.Client, request *model.RestoreRequest) (service.RestoreHandler, error)
}

// restoreManager restores scan backups. The embedded holder keeps the jobs and answers the
// status, cancel and listing calls.
type restoreManager struct {
	*service.RestoreJobsHolder

	executor       RestoreExecutor
	backupReader   service.BackupReader
	clientManager  aerospike.ClientManager
	routineStorage *collections.LockMap
	validator      RestoreValidator
}

var _ service.RestoreManager = (*restoreManager)(nil)

// NewRestoreManager returns the RestoreManager of the scan backup mode.
func NewRestoreManager(
	executor RestoreExecutor,
	clientManager aerospike.ClientManager,
	restoreJobs *service.RestoreJobsHolder,
	backupReader service.BackupReader,
	routineStorage *collections.LockMap,
	validator RestoreValidator,
) service.RestoreManager {
	return &restoreManager{
		RestoreJobsHolder: restoreJobs,
		executor:          executor,
		backupReader:      backupReader,
		clientManager:     clientManager,
		routineStorage:    routineStorage,
		validator:         validator,
	}
}

func (r *restoreManager) Restore(request *model.RestoreRequest) model.RestoreJobID {
	jobID, ctx := r.NewJob(request.BackupDataPath)

	logger := slog.With(slog.Any("jobId", jobID))
	logger.Info("New restore job", slog.Any("path", request.BackupDataPath))
	go func() {
		// finishJob releases the job context, which stops any sub-operation still running.
		err := r.executeRestore(ctx, request, jobID, logger)
		r.FinishJob(jobID, err, logger)
	}()

	return jobID
}

func (r *restoreManager) executeRestore(
	ctx context.Context,
	request *model.RestoreRequest,
	jobID model.RestoreJobID,
	logger *slog.Logger,
) error {
	client, err := r.clientManager.GetClient(ctx, &request.DestinationCluster, nil, logger)
	if err != nil {
		return err
	}
	defer r.clientManager.Close(client)

	// Lock the routine storage from retention manager for the duration of restore.
	// Restore holds RLock to allow concurrent restores for the same routine.
	routineName, _, _ := strings.Cut(request.BackupDataPath, "/") // when restore by path routine is first segment
	if routineName == "" {
		routineName = request.BackupDataPath // fallback to full path
	}
	routineStorageLock := r.routineStorage.Get(routineName)
	routineStorageLock.RLock()
	defer routineStorageLock.RUnlock()

	backups, err := r.backupReader.GetBackups(ctx, service.NewPathFilter(request.BackupDataPath, request.SourceStorage))
	if err != nil {
		return fmt.Errorf("failed to read backups: %w", err)
	}

	if err := r.validator.ValidatePath(
		ctx,
		request,
		client.InfoClient(),
		backups,
	); err != nil {
		return err
	}

	started := time.Now()
	handler, err := r.executor.Run(ctx, client, request)
	if err != nil {
		return err
	}

	r.AddTotalRecords(jobID, recordsInBackup(backups))
	r.AddHandler(jobID, handler)

	if err = handler.Wait(ctx); err != nil {
		return err
	}

	attrs := service.RestoreLogAttrs(handler.Status().Counters, time.Since(started))
	logger.LogAttrs(ctx, slog.LevelInfo, "Finished restoring", attrs...)

	return nil
}

func recordsInBackup(backups []model.BackupDetails) uint64 {
	var records uint64
	for _, b := range backups {
		records += b.RecordCount
	}
	return records
}
