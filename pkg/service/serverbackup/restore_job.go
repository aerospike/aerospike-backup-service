package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// restoreJob follows one server restore job from its start to its end.
type restoreJob struct {
	infoClient backup.ServerBackupInfo
	namespace  string
	jobID      string
	mode       model.RestoreMode
	// stats receives the record counts the server reports for the job.
	stats *models.RestoreStats
}

func newRestoreJob(
	infoClient backup.ServerBackupInfo,
	namespace, jobID string,
	mode model.RestoreMode,
	stats *models.RestoreStats,
) *restoreJob {
	return &restoreJob{infoClient: infoClient, namespace: namespace, jobID: jobID, mode: mode, stats: stats}
}

// Wait polls the job until it is complete and the namespace is open to clients. If the job does
// not complete, or ctx is canceled, Wait releases the job before it returns. The release does not
// use ctx, because a canceled job would otherwise keep running on the server.
func (j *restoreJob) Wait(ctx context.Context) error {
	err := j.poll(ctx, func(status *infomodels.ResponseRestoreState) (bool, error) {
		j.record(status)

		return j.advance(ctx, status)
	})
	if err != nil {
		return errors.Join(err, j.release(context.WithoutCancel(ctx)))
	}

	return nil
}

// record stores the server's record counts in stats. ReadRecords is the number of records the job
// has finished with: written, filtered or failed. RecordsSkipped is the number it read but did
// not write.
func (j *restoreJob) record(status *infomodels.ResponseRestoreState) {
	j.stats.ReadRecords.Store(status.Total)
	j.stats.RecordsSkipped.Store(status.Filtered)
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
		return false, errJobEnded(restoreJobKind, j.jobID, "failed", status.AbortOrigin, status.ErrorReason)
	case status.State == infomodels.RestoreStateAborted:
		return false, errJobEnded(restoreJobKind, j.jobID, "was aborted", status.AbortOrigin, status.ErrorReason)
	case status.HeldReason == heldByTruncateState:
		return false, fmt.Errorf("server restore %s is held: %w", j.jobID, errTruncateState(j.namespace))
	case status.State == infomodels.RestoreStateAborting:
		return false, nil
	case status.Clear == infomodels.ClearStateRequired:
		if err := j.infoClient.ClearRestoreStorage(ctx, j.namespace, j.jobID); err != nil {
			return false, fmt.Errorf("failed to clear namespace %s for server restore %s: %w", j.namespace, j.jobID, err)
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
			fmt.Errorf("failed to open namespace %s after server restore %s: %w", j.namespace, j.jobID, err))
	}

	return abortErr
}

func (j *restoreJob) abort(ctx context.Context) error {
	if err := j.infoClient.AbortRestore(ctx, j.namespace, j.jobID); err != nil {
		return fmt.Errorf("failed to abort server restore %s: %w", j.jobID, err)
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
		return j.infoClient.GetRestoreStatus(ctx, j.namespace, j.jobID)
	}
	_, err := pollJob(ctx, restoreJobKind, j.jobID, fetch, settled)

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
