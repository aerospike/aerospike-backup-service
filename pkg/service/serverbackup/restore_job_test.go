package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// completeStatus is the status of a restore job that completed and whose namespace is open.
var completeStatus = &infomodels.ResponseRestoreState{
	State: infomodels.RestoreStateComplete, NsState: infomodels.NamespaceStateNone,
}

const (
	fullID  = "261020T090111-aaaa"
	incr1ID = "261020T091111-bbbb"
	incr2ID = "261020T092111-cccc"
	// restoreJobID is the id the server restore job is started under.
	restoreJobID = "261105T094929-k9s5"
)

var (
	strictPolicy = model.RestorePolicy{Mode: model.RestoreModeStrict}
	fuzzyPolicy  = model.RestorePolicy{Mode: model.RestoreModeFuzzy}
)

func TestStartRestore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		policy        model.RestorePolicy
		backupIDs     []string
		wantBackupIDs string
		wantPolicy    infomodels.RestorePolicy
	}{
		{name: "strict chain", policy: strictPolicy, backupIDs: []string{fullID, incr1ID, incr2ID},
			wantBackupIDs: fullID + "," + incr1ID + "," + incr2ID, wantPolicy: infomodels.RestorePolicyStrict},
		{name: "strict single backup", policy: strictPolicy, backupIDs: []string{fullID},
			wantBackupIDs: fullID, wantPolicy: infomodels.RestorePolicyStrict},
		{name: "fuzzy chain", policy: fuzzyPolicy, backupIDs: []string{fullID, incr1ID},
			wantBackupIDs: fullID + "," + incr1ID, wantPolicy: infomodels.RestorePolicyFuzzy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			// The restore starts with one command and then follows the job that command returned.
			infoClient := aerospike.NewMockInfoGetter(ctrl)
			gomock.InOrder(
				infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, request *infomodels.RequestRestore) (string, error) {
						assert.Equal(t, tt.wantBackupIDs, request.BackupIDs)
						assert.Equal(t, "bar", request.Namespace)
						assert.Equal(t, "backups/routine", request.Path, "the routine's folder in the storage")
						assert.Equal(t, tt.wantPolicy, request.Policy)

						return restoreJobID, nil
					}),
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", restoreJobID).Return(completeStatus, nil),
			)

			backups := make([]model.BackupDetails, 0, len(tt.backupIDs))
			for _, id := range tt.backupIDs {
				backups = append(backups, chainBackup(id))
			}

			startAndWait(t, infoClient, tt.policy, backups...)
		})
	}
}

func TestStartRestore_SkipsIndexesAndUDFs(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *infomodels.RequestRestore) (string, error) {
			assert.Equal(t, new(true), request.NoIndexes)
			assert.Equal(t, new(true), request.NoUDFs)

			return restoreJobID, nil
		})
	infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", restoreJobID).Return(completeStatus, nil)

	policy := fuzzyPolicy
	policy.NoIndexes = new(true)
	policy.NoUdfs = new(true)

	startAndWait(t, infoClient, policy, chainBackup(fullID))
}

// TestStartRestore_Retry checks that the server gets the retry settings that the policy of a fuzzy
// restore sets, and no others.
func TestStartRestore_Retry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		policy *model.RetryPolicy
		want   infomodels.RequestRestore
	}{
		{
			name: "every setting",
			policy: &model.RetryPolicy{
				BaseTimeout: optional.Of(1500 * time.Millisecond),
				Multiplier:  new(2.0),
				MaxRetries:  optional.Of(0),
			},
			want: infomodels.RequestRestore{
				RetryBaseIntervalMs: new(1500), RetryMultiplier: 2, RetryMaxAttempts: new(0),
			},
		},
		{
			name:   "only a multiplier",
			policy: &model.RetryPolicy{Multiplier: new(3.0)},
			want:   infomodels.RequestRestore{RetryMultiplier: 3},
		},
		{name: "no policy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			infoClient := aerospike.NewMockInfoGetter(ctrl)
			infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, request *infomodels.RequestRestore) (string, error) {
					assert.Equal(t, tt.want.RetryBaseIntervalMs, request.RetryBaseIntervalMs)
					assert.InDelta(t, tt.want.RetryMultiplier, request.RetryMultiplier, 0)
					assert.Equal(t, tt.want.RetryMaxAttempts, request.RetryMaxAttempts)

					return restoreJobID, nil
				})
			infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", restoreJobID).Return(completeStatus, nil)

			policy := fuzzyPolicy
			policy.RetryPolicy = tt.policy
			startAndWait(t, infoClient, policy, chainBackup(fullID))
		})
	}
}

// TestStartRestore_StartRefused checks that a refused start fails the restore, explains a refusal
// for truncate state, and cleans nothing up, because a refused job never started.
func TestStartRestore_StartRefused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  model.RestorePolicy
		refusal string
		wantErr string
	}{
		{name: "strict", policy: strictPolicy, refusal: "refused",
			wantErr: "failed to start server restore: refused"},
		{name: "fuzzy", policy: fuzzyPolicy, refusal: "refused",
			wantErr: "failed to start server restore: refused"},
		{
			name:    "truncate state",
			policy:  strictPolicy,
			refusal: "ERROR:22:restore: ns=bar holds truncate state - see the log to clear it",
			wantErr: "clear it with the info command 'truncate-namespace-undo:namespace=bar'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			infoClient := aerospike.NewMockInfoGetter(ctrl)
			// The start returns the job id with every error. A refused job is not cleaned up.
			infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).Return(restoreJobID, errors.New(tt.refusal))

			_, err := startRestore(t.Context(), infoClient, tt.policy, []model.BackupDetails{chainBackup(fullID)})
			require.ErrorContains(t, err, tt.wantErr)
			require.ErrorContains(t, err, tt.refusal)
		})
	}
}

// TestStartRestore_StartOutcomeUnknown checks that a start whose outcome is unknown releases the
// job it may have started: a strict job is aborted and its namespace opened, a fuzzy job is only
// aborted.
func TestStartRestore_StartOutcomeUnknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		policy model.RestorePolicy
		expect func(infoClient *aerospike.MockInfoGetter)
	}{
		{name: "strict", policy: strictPolicy, expect: func(infoClient *aerospike.MockInfoGetter) {
			expectRelease(infoClient, restoreJobID)
		}},
		{name: "fuzzy", policy: fuzzyPolicy, expect: func(infoClient *aerospike.MockInfoGetter) {
			infoClient.EXPECT().AbortRestore(gomock.Any(), "bar", restoreJobID).Return(nil)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			// The server may have started the job even though the start failed.
			startErr := fmt.Errorf("%w: timeout", asinfo.ErrOutcomeUnknown)
			infoClient := aerospike.NewMockInfoGetter(ctrl)
			infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).Return(restoreJobID, startErr)
			tt.expect(infoClient)

			_, err := startRestore(t.Context(), infoClient, tt.policy, []model.BackupDetails{chainBackup(fullID)})
			require.ErrorIs(t, err, asinfo.ErrOutcomeUnknown)
		})
	}
}

// TestStartRestore_InvalidBackupKey checks that a backup whose key names no routine is refused
// before anything is sent to the server.
func TestStartRestore_InvalidBackupKey(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	backup := chainBackup(fullID)
	backup.Key = fullID // a bare backup id names no routine.

	_, err := startRestore(t.Context(), aerospike.NewMockInfoGetter(ctrl), strictPolicy,
		[]model.BackupDetails{backup})
	require.EqualError(t, err, `invalid server backup key "`+fullID+`": want <routine>/<job id>`)
}

func TestStartRestore_ChainTooLong(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	backups := make([]model.BackupDetails, maxChainLength+1)
	for i := range backups {
		backups[i] = chainBackup(fmt.Sprintf("261020T09%04d-aaaa", i))
	}

	_, err := startRestore(t.Context(), aerospike.NewMockInfoGetter(ctrl), strictPolicy, backups)
	require.EqualError(t, err, "restore chain has 33 backups, the server restores at most 32 in one job")
}

// newTestRestoreJob returns the restore job "job" of namespace bar.
func newTestRestoreJob(infoClient *aerospike.MockInfoGetter, mode model.RestoreMode) *restoreJob {
	return &restoreJob{infoClient: infoClient, namespace: "bar", serverJobID: "job", mode: mode}
}

func TestRestoreJob_Wait_UntilNamespaceOpen(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	gomock.InOrder(
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(&infomodels.ResponseRestoreState{
			State:   infomodels.RestoreStateComplete,
			NsState: infomodels.NamespaceStateOpening,
		}, nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(completeStatus, nil),
	)

	require.NoError(t, newTestRestoreJob(infoClient, model.RestoreModeStrict).Wait(t.Context()))
}

// TestRestoreJob_Wait_ClearsNamespace checks that a strict restore clears the storage of a node
// that reports it must be cleared, and then goes on waiting for the job.
func TestRestoreJob_Wait_ClearsNamespace(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	preparing := func(clearState infomodels.ClearState) *infomodels.ResponseRestoreState {
		return &infomodels.ResponseRestoreState{
			State: infomodels.RestoreStateActive, NsState: infomodels.NamespaceStatePreparing, Clear: clearState,
		}
	}

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	gomock.InOrder(
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(
			preparing(infomodels.ClearStateRequired), nil),
		infoClient.EXPECT().ClearRestoreStorage(gomock.Any(), "bar", "job").Return(nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(
			preparing(infomodels.ClearStateClearing), nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(completeStatus, nil),
	)

	require.NoError(t, newTestRestoreJob(infoClient, model.RestoreModeStrict).Wait(t.Context()))
}

// TestRestoreJob_Wait_RecordsCounts checks that the job's status holds the record counts of the
// last status the server reported.
func TestRestoreJob_Wait_RecordsCounts(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	gomock.InOrder(
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(&infomodels.ResponseRestoreState{
			State: infomodels.RestoreStateActive, Total: 4, Filtered: 1,
		}, nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(&infomodels.ResponseRestoreState{
			State: infomodels.RestoreStateComplete, NsState: infomodels.NamespaceStateNone, Total: 10, Filtered: 3,
		}, nil),
	)

	job := newTestRestoreJob(infoClient, model.RestoreModeFuzzy)
	require.NoError(t, job.Wait(t.Context()))

	counters := job.Status().Counters
	assert.Equal(t, uint64(10), counters.ReadRecords)
	assert.Equal(t, uint64(3), counters.SkippedRecords)
}

// TestRestoreJob_Wait_ReleasesNamespace checks that a strict restore that does not complete is
// released, whatever stopped it.
func TestRestoreJob_Wait_ReleasesNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		status    *infomodels.ResponseRestoreState
		statusErr error
		// clearErr is the answer to the clear that the status asks for.
		clearErr error
		wantErr  string
	}{
		{
			name:    "failed",
			status:  &infomodels.ResponseRestoreState{State: infomodels.RestoreStateFailed, ErrorReason: "s3 GET 403"},
			wantErr: "server restore job failed: s3 GET 403",
		},
		{
			name: "aborted",
			status: &infomodels.ResponseRestoreState{
				State: infomodels.RestoreStateAborted, ErrorReason: "node left", AbortOrigin: infomodels.AbortOriginCluster,
			},
			wantErr: "server restore job was aborted (abort origin: cluster): node left",
		},
		{
			// No clear is sent: the job has ended and would restore nothing into the cleared storage.
			name: "failed while a node must be cleared",
			status: &infomodels.ResponseRestoreState{
				State: infomodels.RestoreStateFailed, Clear: infomodels.ClearStateRequired,
			},
			wantErr: "server restore job failed",
		},
		{
			name:      "status unavailable",
			statusErr: errors.New("timeout"),
			wantErr:   "failed to get status of server restore job: timeout",
		},
		{
			name: "clear refused",
			status: &infomodels.ResponseRestoreState{
				NsState: infomodels.NamespaceStatePreparing, Clear: infomodels.ClearStateRequired,
			},
			clearErr: errors.New("refused"),
			wantErr:  "failed to clear namespace bar for server restore job: refused",
		},
		{
			name: "held by truncate state",
			status: &infomodels.ResponseRestoreState{
				NsState: infomodels.NamespaceStatePreparing, HeldReason: heldByTruncateState,
			},
			wantErr: "clear it with the info command 'truncate-namespace-undo:namespace=bar'",
		},
		{
			// The held node does not end by itself, so the job is released instead of waited for.
			name: "held by truncate state while another node aborts",
			status: &infomodels.ResponseRestoreState{
				State:      infomodels.RestoreStateAborting,
				NsState:    infomodels.NamespaceStatePreparing,
				HeldReason: heldByTruncateState,
			},
			wantErr: "clear it with the info command 'truncate-namespace-undo:namespace=bar'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			infoClient := aerospike.NewMockInfoGetter(ctrl)
			infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(tt.status, tt.statusErr)
			if tt.clearErr != nil {
				infoClient.EXPECT().ClearRestoreStorage(gomock.Any(), "bar", "job").Return(tt.clearErr)
			}
			expectRelease(infoClient, "job")

			err := newTestRestoreJob(infoClient, model.RestoreModeStrict).Wait(t.Context())
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestRestoreJob_Wait_Canceled checks that a canceled strict restore is still released: the job
// keeps running on the server and keeps the namespace closed.
func TestRestoreJob_Wait_Canceled(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	expectRelease(infoClient, "job")

	require.ErrorIs(t, newTestRestoreJob(infoClient, model.RestoreModeStrict).Wait(ctx), context.Canceled)
}

// TestRestoreJob_Wait_FuzzyAbortsOnly checks that a fuzzy restore that does not complete is
// aborted and that its namespace, which it never closed, is not opened.
func TestRestoreJob_Wait_FuzzyAbortsOnly(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(
		&infomodels.ResponseRestoreState{State: infomodels.RestoreStateFailed}, nil)
	infoClient.EXPECT().AbortRestore(gomock.Any(), "bar", "job").Return(nil)

	err := newTestRestoreJob(infoClient, model.RestoreModeFuzzy).Wait(t.Context())
	require.ErrorContains(t, err, "server restore job failed")
}

// TestRestoreJob_Wait_OpensAfterRefusedAbort checks that the namespace of a strict restore that
// ended by itself is opened even though the server refuses to abort the job.
func TestRestoreJob_Wait_OpensAfterRefusedAbort(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	failed := &infomodels.ResponseRestoreState{
		State: infomodels.RestoreStateFailed, NsState: infomodels.NamespaceStateFailed,
	}
	infoClient := aerospike.NewMockInfoGetter(ctrl)
	gomock.InOrder(
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(failed, nil),
		infoClient.EXPECT().AbortRestore(gomock.Any(), "bar", "job").Return(errors.New("job is not active")),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(failed, nil),
		infoClient.EXPECT().OpenRestore(gomock.Any(), "bar").Return(nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(&infomodels.ResponseRestoreState{
			State: infomodels.RestoreStateFailed, NsState: infomodels.NamespaceStateNone,
		}, nil),
	)

	err := newTestRestoreJob(infoClient, model.RestoreModeStrict).Wait(t.Context())

	require.ErrorContains(t, err, "server restore job failed")
	require.ErrorContains(t, err, "failed to abort server restore job: job is not active")
	require.NotContains(t, err.Error(), "failed to open namespace")
}

// TestRestoreJob_Wait_LostJob checks that a job the cluster stops reporting is given up on and
// released. A lost job refuses the abort and is never seen to end, so its namespace is not opened.
//
//nolint:paralleltest // shortens lostJobTimeout, which the parallel tests read.
func TestRestoreJob_Wait_LostJob(t *testing.T) {
	shortenLostJobTimeout(t)
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	gomock.InOrder(
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(nil, asinfo.ErrNotFound).Times(2),
		infoClient.EXPECT().AbortRestore(gomock.Any(), "bar", "job").Return(asinfo.ErrNotFound),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(nil, asinfo.ErrNotFound).Times(2),
	)

	err := newTestRestoreJob(infoClient, model.RestoreModeStrict).Wait(t.Context())

	require.ErrorContains(t, err, "server restore job not reported for 1.5s")
	require.ErrorContains(t, err, "failed to abort server restore job")
	require.ErrorContains(t, err, "failed to open namespace bar after server restore job: the job did not end")
}

// startAndWait starts the restore of the backups and waits for it to end without error.
func startAndWait(
	t *testing.T,
	infoClient *aerospike.MockInfoGetter,
	policy model.RestorePolicy,
	backups ...model.BackupDetails,
) {
	t.Helper()

	job, err := startRestore(t.Context(), infoClient, policy, backups)
	require.NoError(t, err)
	require.NoError(t, job.Wait(t.Context()))
}

// expectRelease expects the cleanup of a strict restore of namespace bar: abort the job, poll
// until it has ended, open the namespace, poll until it is open.
func expectRelease(infoClient *aerospike.MockInfoGetter, jobID string) {
	gomock.InOrder(
		infoClient.EXPECT().AbortRestore(gomock.Any(), "bar", jobID).Return(nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", jobID).Return(
			&infomodels.ResponseRestoreState{State: infomodels.RestoreStateAborting}, nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", jobID).Return(
			&infomodels.ResponseRestoreState{State: infomodels.RestoreStateAborted}, nil),
		// restore-open takes only the namespace: a namespace has at most one restore job.
		infoClient.EXPECT().OpenRestore(gomock.Any(), "bar").Return(nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", jobID).Return(
			&infomodels.ResponseRestoreState{
				State: infomodels.RestoreStateAborted, NsState: infomodels.NamespaceStateOpening,
			}, nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", jobID).Return(
			&infomodels.ResponseRestoreState{
				State: infomodels.RestoreStateAborted, NsState: infomodels.NamespaceStateNone,
			}, nil),
	)
}

// chainBackup returns the details of a backup of namespace bar, as the catalog lists it.
func chainBackup(backupID string) model.BackupDetails {
	return model.BackupDetails{
		BackupMetadata: model.BackupMetadata{Namespace: "bar"},
		Key:            backupKey("routine", backupID),
		Storage:        &model.S3Storage{Bucket: "bucket", S3Region: "us-east-1", Path: "backups"},
	}
}
