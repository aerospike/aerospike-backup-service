package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
)

type pathRestoreRunner struct {
	restoreJobs    *RestoreJobsHolder
	restoreService restoreexecutor.Restore
	backupReader   BackupReader
	clientManager  aerospike.ClientManager
	routineStorage *collections.LockMap
	validator      RestoreValidator
}

func newPathRestoreRunner(
	restoreJobs *RestoreJobsHolder,
	restoreService restoreexecutor.Restore,
	backupReader BackupReader,
	clientManager aerospike.ClientManager,
	routineStorage *collections.LockMap,
	validator RestoreValidator,
) *pathRestoreRunner {
	return &pathRestoreRunner{
		restoreJobs:    restoreJobs,
		restoreService: restoreService,
		backupReader:   backupReader,
		clientManager:  clientManager,
		routineStorage: routineStorage,
		validator:      validator,
	}
}

func (r *pathRestoreRunner) Restore(ctx context.Context, request *model.RestoreRequest) (model.RestoreJobID, error) {
	// Create a cancellable context for this specific job.
	ctx, cancel := context.WithCancel(ctx)

	jobID := r.restoreJobs.newJob(request.BackupDataPath, cancel)
	logger := slog.With(slog.Any("jobId", jobID))
	go func() {
		err := r.executeRestore(ctx, request, jobID, logger)
		if err != nil { // if some of the restore sub-operations failed, we need to cancel the rest.
			cancel()
		}
		r.restoreJobs.finishJob(jobID, err, logger)
	}()

	return jobID, nil
}

func (r *pathRestoreRunner) executeRestore(
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

	backups, err := r.backupReader.GetBackups(ctx, NewPathFilter(request.BackupDataPath, request.SourceStorage))
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

	handler, err := r.restoreService.Run(ctx, client, request)
	if err != nil {
		return fmt.Errorf("failed to start restore operation: %w", err)
	}
	logger.Info("Start restoring", slog.Any("backup", backups))

	r.restoreJobs.addTotalRecords(jobID, recordsInBackup(backups))
	r.restoreJobs.addHandler(jobID, handler)

	if err = handler.Wait(ctx); err != nil {
		return err
	}

	logger.LogAttrs(ctx, slog.LevelInfo, "Finished restoring", logAttrs(handler.GetStats())...)

	return nil
}

func recordsInBackup(backups []model.BackupDetails) uint64 {
	var records uint64
	for _, b := range backups {
		records += b.RecordCount
	}
	return records
}
