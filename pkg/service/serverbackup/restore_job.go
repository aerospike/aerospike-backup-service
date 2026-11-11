package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// startRestore starts one server restore job that applies backups, the backups of one namespace
// oldest first, and returns the job to follow. The server writes the backed-up records into the
// live namespace (the fuzzy restore policy), which clients keep using.
func startRestore(
	ctx context.Context,
	infoClient backup.ServerBackupInfo,
	policy model.RestorePolicy,
	backups []model.BackupDetails,
) (*restoreJob, error) {
	first := backups[0]
	routineName, backupIDs, err := backupIDsOf(backups)
	if err != nil {
		return nil, err
	}

	// The server writes into the namespace the backup was taken from. A remap is refused, so
	// that nothing is written into a namespace the request did not ask for.
	namespace := first.Namespace
	if destination := policy.Namespace.DestinationOr(namespace); destination != namespace {
		return nil, fmt.Errorf("server restore cannot restore namespace %s into %s", namespace, destination)
	}

	common, err := withStorage(infomodels.RequestCommon{Namespace: namespace}, first.Storage, routineName)
	if err != nil {
		return nil, err
	}

	jobID, err := infoClient.StartRestore(ctx, &infomodels.RequestRestore{
		RequestCommon: common,
		BackupIDs:     strings.Join(backupIDs, ","),
		Policy:        infomodels.RestorePolicyFuzzy,
	})
	job := &restoreJob{infoClient: infoClient, namespace: namespace, serverJobID: jobID}
	if err != nil {
		startErr := fmt.Errorf("failed to start server restore: %w", err)
		if !errors.Is(err, asinfo.ErrOutcomeUnknown) {
			return nil, startErr
		}

		// The server may have started the job though the start failed.
		return nil, errors.Join(startErr, job.abort(ctx))
	}

	return job, nil
}

// backupIDsOf returns the routine whose folder holds the backups, and their ids in the given
// order. A backup's id is the id of the server job that wrote it. A chain is backed up by one
// routine, so the first key names the routine.
func backupIDsOf(backups []model.BackupDetails) (string, []string, error) {
	routineName, _, err := parseBackupKey(backups[0].Key)
	if err != nil {
		return "", nil, err
	}

	backupIDs := make([]string, 0, len(backups))
	for _, b := range backups {
		_, backupID, err := parseBackupKey(b.Key)
		if err != nil {
			return "", nil, err
		}
		backupIDs = append(backupIDs, backupID)
	}

	return routineName, backupIDs, nil
}

// restoreJob follows a server restore job. The restore manager registers it as the handler of
// the restore job it belongs to.
type restoreJob struct {
	infoClient  backup.ServerBackupInfo
	namespace   string
	serverJobID string
}

var _ service.RestoreHandler = (*restoreJob)(nil)

// Status returns an empty run, because the job does not read the counts or the progress the
// server reports.
func (h *restoreJob) Status() model.RestoreRun {
	return model.RestoreRun{}
}

// Wait polls the restore job until it ends, or ctx is canceled. A restore that does not complete
// is aborted, even when ctx is canceled, so that it stops writing into the namespace.
func (h *restoreJob) Wait(ctx context.Context) error {
	err := h.awaitComplete(ctx)
	if err == nil {
		return nil
	}

	return errors.Join(err, h.abort(ctx))
}

// abort aborts the job, even when ctx is canceled, so that it stops writing into the namespace.
// A failed abort is returned: the job may still be writing.
func (h *restoreJob) abort(ctx context.Context) error {
	if err := h.infoClient.AbortRestore(context.WithoutCancel(ctx), h.namespace, h.serverJobID); err != nil {
		return fmt.Errorf("failed to abort server restore %s: %w", h.serverJobID, err)
	}

	return nil
}

// awaitComplete polls the restore job until it completes. It fails when the job fails or is
// aborted, or ctx is canceled.
func (h *restoreJob) awaitComplete(ctx context.Context) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	seen := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		status, err := h.infoClient.GetRestoreStatus(ctx, h.namespace, h.serverJobID)
		if errors.Is(err, asinfo.ErrNotFound) && !seen {
			// The job is not visible on every node yet. Once it has been, a node that does not
			// know it (one that joined or restarted) means the job can no longer be followed.
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to get server restore status: %w", err)
		}
		seen = true

		switch status.State {
		case infomodels.RestoreStateComplete:
			return nil
		case infomodels.RestoreStateFailed:
			return fmt.Errorf("server restore %s failed", h.serverJobID)
		case infomodels.RestoreStateAborted:
			return fmt.Errorf("server restore %s was aborted", h.serverJobID)
		default:
		}
	}
}
