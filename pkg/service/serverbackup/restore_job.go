package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// maxChainLength is the most backups the server restores in one job (AS_RESTORE_CHAIN_MAX_LINKS).
const maxChainLength = 32

// startRestore starts one server restore job that applies backups, the backups of one namespace
// oldest first, and returns the job to follow. The policy's mode selects a strict or a fuzzy
// restore.
func startRestore(
	ctx context.Context,
	infoClient backup.ServerBackupInfo,
	policy model.RestorePolicy,
	backups []model.BackupDetails,
) (*restoreJob, error) {
	if len(backups) > maxChainLength {
		return nil, fmt.Errorf("restore chain has %d backups, the server restores at most %d in one job",
			len(backups), maxChainLength)
	}

	first := backups[0]
	routineName, backupIDs, err := backupIDsOf(backups)
	if err != nil {
		return nil, err
	}

	// The DTO layer rejects a namespace mapping, so the job restores into the backup's namespace.
	namespace := first.Namespace
	common, err := withStorage(infomodels.RequestCommon{
		Namespace: namespace,
		SetList:   strings.Join(policy.SetList, ","),
		NoIndexes: policy.NoIndexes,
		NoUDFs:    policy.NoUdfs,
	}, first.Storage, routineName)
	if err != nil {
		return nil, err
	}

	// An unset limit is sent as zero. The info client leaves zero fields out of the command, so
	// the server uses its own default. The retry settings are set only for a fuzzy restore,
	// because the DTO layer rejects a retry policy on a strict one.
	retry := policy.RetryPolicy
	jobID, err := infoClient.StartRestore(ctx, &infomodels.RequestRestore{
		RequestCommon:        common,
		BackupIDs:            strings.Join(backupIDs, ","),
		RestoreWorkerThreads: ptr.ValueOrZero(policy.Parallel),
		RestoreRPS:           ptr.ValueOrZero(policy.Tps),
		ExtraTTL:             int(ptr.ValueOrZero(policy.ExtraTTL)),
		// The server's restore policies have the names of the restore modes.
		Policy:              infomodels.RestorePolicy(policy.Mode),
		RetryBaseIntervalMs: retry.BaseTimeoutMsOrNil(),
		RetryMultiplier:     retry.MultiplierOrZero(),
		RetryMaxAttempts:    retry.MaxRetriesOrNil(),
	})

	job := &restoreJob{infoClient: infoClient, namespace: namespace, serverJobID: jobID, mode: policy.Mode}
	if err != nil {
		startErr := fmt.Errorf("failed to start server restore: %w", explainRefusal(err, namespace))
		if !errors.Is(err, asinfo.ErrOutcomeUnknown) {
			return nil, startErr
		}

		// The server may have started the job even though the start failed.
		return nil, errors.Join(startErr, job.release(context.WithoutCancel(ctx)))
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

// restoreJob follows one server restore job from its start to its end. The restore manager
// registers it as the handler of the restore job it belongs to.
type restoreJob struct {
	infoClient  backup.ServerBackupInfo
	namespace   string
	serverJobID string
	mode        model.RestoreMode
	// readRecords and skippedRecords are the record counts of the last status the server
	// reported. Status reads them while Wait polls.
	readRecords    atomic.Uint64
	skippedRecords atomic.Uint64
}

var _ service.RestoreHandler = (*restoreJob)(nil)

// Status returns the record counts the server last reported for the job. ReadRecords is the
// number of records the job has finished with: written, filtered or failed. SkippedRecords is the
// number it read but did not write.
func (j *restoreJob) Status() model.RestoreRun {
	return model.RestoreRun{Counters: model.RestoreCounters{
		ReadRecords:    j.readRecords.Load(),
		SkippedRecords: j.skippedRecords.Load(),
	}}
}

// Wait polls the job until it is complete and the namespace is open to clients. If the job does
// not complete, or ctx is canceled, Wait releases the job before it returns. The release does not
// use ctx, because a canceled job would otherwise keep running on the server.
func (j *restoreJob) Wait(ctx context.Context) error {
	err := j.poll(ctx, func(status *infomodels.ResponseRestoreState) (bool, error) {
		j.readRecords.Store(status.Total)
		j.skippedRecords.Store(status.Filtered)

		return j.advance(ctx, status)
	})
	if err != nil {
		return errors.Join(err, j.release(context.WithoutCancel(ctx)))
	}

	return nil
}

// advance handles one status of the job. It returns true once the job is complete and the server
// has opened the namespace to clients. It returns an error when the job failed, was aborted, or
// is blocked by truncate state.
//
// During a strict restore a node that still has data in the namespace reports that its storage
// must be cleared, and advance sends that clear. The failed, aborted and aborting cases come
// first on purpose: clearing storage for a job that is ending would erase data and restore
// nothing in its place.
func (j *restoreJob) advance(ctx context.Context, status *infomodels.ResponseRestoreState) (bool, error) {
	switch {
	case status.State == infomodels.RestoreStateFailed:
		return false, errJobEnded(restoreJobKind, j.serverJobID, "failed", status.AbortOrigin, status.ErrorReason)
	case status.State == infomodels.RestoreStateAborted:
		return false, errJobEnded(restoreJobKind, j.serverJobID, "was aborted", status.AbortOrigin, status.ErrorReason)
	case status.HeldReason == heldByTruncateState:
		return false, fmt.Errorf("server restore %s is held: %w", j.serverJobID, errTruncateState(j.namespace))
	case status.State == infomodels.RestoreStateAborting:
		return false, nil
	case status.Clear == infomodels.ClearStateRequired:
		if err := j.infoClient.ClearRestoreStorage(ctx, j.namespace, j.serverJobID); err != nil {
			return false, fmt.Errorf("failed to clear namespace %s for server restore %s: %w",
				j.namespace, j.serverJobID, err)
		}

		return false, nil
	default:
		// The job is complete before the server opens the namespace to clients.
		return status.State == infomodels.RestoreStateComplete && status.NsState == infomodels.NamespaceStateNone, nil
	}
}

// release cleans up after a job that did not complete: it aborts the job. A strict restore keeps
// the namespace closed to clients after an abort, so release then opens the namespace, with
// whatever the restore wrote. A fuzzy restore never closes the namespace.
func (j *restoreJob) release(ctx context.Context) error {
	abortErr := j.abort(ctx)
	if j.mode == model.RestoreModeFuzzy {
		return abortErr
	}

	// Continue when the abort fails. A job that ended by itself may refuse the abort, and it
	// still keeps the namespace closed.
	if err := j.openNamespace(ctx); err != nil {
		return errors.Join(abortErr,
			fmt.Errorf("failed to open namespace %s after server restore %s: %w", j.namespace, j.serverJobID, err))
	}

	return abortErr
}

func (j *restoreJob) abort(ctx context.Context) error {
	if err := j.infoClient.AbortRestore(ctx, j.namespace, j.serverJobID); err != nil {
		return fmt.Errorf("failed to abort server restore %s: %w", j.serverJobID, err)
	}

	return nil
}

// openNamespace sends restore-open once the job has ended, then waits until the namespace is
// open. The server applies an abort and an open asynchronously. A node that finishes aborting
// after restore-open closes the namespace again, so the job must have ended on every node first.
func (j *restoreJob) openNamespace(ctx context.Context) error {
	if err := j.pollUntil(ctx, "end", (*infomodels.ResponseRestoreState).IsFinished); err != nil {
		return err
	}

	if err := j.infoClient.OpenRestore(ctx, j.namespace); err != nil {
		return err
	}

	open := func(status *infomodels.ResponseRestoreState) bool {
		return status.NsState == infomodels.NamespaceStateNone
	}

	return j.pollUntil(ctx, "open the namespace", open)
}

// pollUntil polls the job for at most cleanupTimeout, until done accepts its status. step says
// what the job is waited to do, as the error words it: "end", "open the namespace".
func (j *restoreJob) pollUntil(
	ctx context.Context,
	step string,
	done func(*infomodels.ResponseRestoreState) bool,
) error {
	ctx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()

	err := j.poll(ctx, func(status *infomodels.ResponseRestoreState) (bool, error) { return done(status), nil })
	if err != nil {
		return fmt.Errorf("the job did not %s: %w", step, err)
	}

	return nil
}

// poll polls the job until settled reports that the wait is over.
func (j *restoreJob) poll(ctx context.Context, settled func(*infomodels.ResponseRestoreState) (bool, error)) error {
	fetch := func(ctx context.Context) (*infomodels.ResponseRestoreState, error) {
		return j.infoClient.GetRestoreStatus(ctx, j.namespace, j.serverJobID)
	}
	_, err := pollJob(ctx, restoreJobKind, j.serverJobID, fetch, settled)

	return err
}

// truncateStateRefusal is the text in the server's error when it refuses a strict restore because
// the namespace has truncate state. A truncate hides every record last updated before it.
// Restored records keep the update times they had in the backup, so the truncate would hide them.
const truncateStateRefusal = "holds truncate state"

// heldByTruncateState is the held-reason a node reports when a truncate was committed after the
// strict restore started. The node keeps the namespace in preparation and the job cannot go on.
const heldByTruncateState = "truncate-state-live"

// explainRefusal adds the remedy to a truncate-state refusal, whose own text only points to the
// server log.
func explainRefusal(err error, namespace string) error {
	if !strings.Contains(err.Error(), truncateStateRefusal) {
		return err
	}

	return fmt.Errorf("%w: %w", errTruncateState(namespace), err)
}

// errTruncateState tells the user how to clear the truncate state that blocks a strict restore
// into namespace.
func errTruncateState(namespace string) error {
	return fmt.Errorf("namespace %s holds truncate state, which would filter out the restored records: "+
		"clear it with the info command 'truncate-namespace-undo:namespace=%s', or "+
		"'truncate-undo:namespace=%s;set=<set>' for each truncated set the server log names, and retry",
		namespace, namespace, namespace)
}
