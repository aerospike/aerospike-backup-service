package serverbackup

import (
	"context"
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// newTestRestoreJob returns the restore job "job" of namespace bar.
func newTestRestoreJob(infoClient *aerospike.MockInfoGetter, mode model.RestoreMode) *restoreJob {
	return newRestoreJob(infoClient, "bar", "job", mode, models.NewRestoreStats())
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

// TestRestoreJob_Wait_RecordsCounts checks that the job's stats hold the record counts of the last
// status the server reported.
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

	assert.Equal(t, uint64(10), job.stats.GetReadRecords())
	assert.Equal(t, uint64(3), job.stats.GetRecordsSkipped())
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
