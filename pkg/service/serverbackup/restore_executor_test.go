package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	fullID  = "261020T090111-aaaa"
	incr1ID = "261020T091111-bbbb"
	incr2ID = "261020T092111-cccc"
	// restoreJobID is the id the server restore job is started under.
	restoreJobID = "261105T094929-k9s5"
)

func TestRestoreExecutor_Run_Cold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		backupIDs     []string
		wantBackupIDs string
	}{
		{name: "chain", backupIDs: []string{fullID, incr1ID, incr2ID}, wantBackupIDs: fullID + "," + incr1ID + "," + incr2ID},
		{name: "single backup", backupIDs: []string{fullID}, wantBackupIDs: fullID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			// One command starts a cold restore: the server prepares the namespace itself. The
			// restore then follows the job the start returned.
			infoClient := aerospike.NewMockInfoGetter(ctrl)
			gomock.InOrder(
				infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, request *infomodels.RequestRestore) (string, error) {
						assert.Equal(t, tt.wantBackupIDs, request.BackupIDs)
						assert.Equal(t, "bar", request.Namespace)
						assert.Equal(t, infomodels.RestorePolicyStrict, request.Policy)

						return restoreJobID, nil
					}),
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", restoreJobID).Return(
					&infomodels.ResponseRestoreState{
						State: infomodels.RestoreStateComplete, NsState: infomodels.NamespaceStateNone,
					}, nil),
			)

			requests := make([]*model.RestoreRequest, 0, len(tt.backupIDs))
			for _, id := range tt.backupIDs {
				requests = append(requests, chainRequest(id))
			}

			handler, err := NewRestoreExecutor(namespaceReader(t, ctrl)).
				Run(t.Context(), clientOf(ctrl, infoClient), requests[0], requests[1:]...)
			require.NoError(t, err)
			require.NoError(t, handler.Wait(t.Context()))
		})
	}
}

// TestRestoreExecutor_Run_StartRefused checks that a refused start fails the restore, explains a
// refusal for truncate state, and releases nothing: the refused job gates nothing.
func TestRestoreExecutor_Run_StartRefused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request *model.RestoreRequest
		refusal string
		wantErr string
	}{
		{name: "strict", request: chainRequest(fullID), refusal: "refused",
			wantErr: "failed to start server restore: refused"},
		{name: "fuzzy", request: fuzzyRequest(fullID), refusal: "refused",
			wantErr: "failed to start server restore: refused"},
		{
			name:    "truncate state",
			request: chainRequest(fullID),
			refusal: "ERROR:22:restore: ns=bar holds truncate state - see the log to clear it",
			wantErr: "clear it with the info command 'truncate-namespace-undo:namespace=bar'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			infoClient := aerospike.NewMockInfoGetter(ctrl)
			// The server may have accepted a start that failed, so it returns the job id anyway.
			infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).Return(restoreJobID, errors.New(tt.refusal))

			handler, err := NewRestoreExecutor(namespaceReader(t, ctrl)).Run(t.Context(), clientOf(ctrl, infoClient),
				tt.request)
			require.ErrorContains(t, err, tt.wantErr)
			require.ErrorContains(t, err, tt.refusal)
			assert.Nil(t, handler)
		})
	}
}

// expectRelease expects the job *jobID of namespace bar to be aborted, then its namespace opened.
func expectRelease(t *testing.T, infoClient *aerospike.MockInfoGetter, jobID *string) {
	t.Helper()

	gomock.InOrder(
		infoClient.EXPECT().AbortRestore(gomock.Any(), "bar", gomock.Any()).DoAndReturn(
			func(_ context.Context, _, id string) error {
				assert.Equal(t, *jobID, id)

				return nil
			}),
		infoClient.EXPECT().OpenRestore(gomock.Any(), "bar", gomock.Any()).DoAndReturn(
			func(_ context.Context, _, id string) error {
				assert.Equal(t, *jobID, id)

				return nil
			}),
	)
}

func TestRestoreExecutor_Run_ChainTooLong(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	requests := make([]*model.RestoreRequest, maxChainLength+1)
	for i := range requests {
		requests[i] = chainRequest(fmt.Sprintf("261020T09%04d-aaaa", i))
	}

	_, err := NewRestoreExecutor(service.NewMockBackupReader(ctrl)).
		Run(t.Context(), aerospike.NewMockClient(ctrl), requests[0], requests[1:]...)
	require.ErrorContains(t, err, "at most 32 backups")
}

// namespaceReader returns a BackupReader that reports the full backup in namespace bar.
func namespaceReader(t *testing.T, ctrl *gomock.Controller) service.BackupReader {
	t.Helper()

	backupReader := service.NewMockBackupReader(ctrl)
	backupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, filter service.BackupFilter) ([]model.BackupDetails, error) {
			assert.Equal(t, fullID, filter.(*service.PathFilter).Path(), "the namespace comes from the full backup")

			return []model.BackupDetails{{BackupMetadata: model.BackupMetadata{Namespace: "bar"}}}, nil
		})

	return backupReader
}

func clientOf(ctrl *gomock.Controller, infoClient *aerospike.MockInfoGetter) aerospike.Client {
	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().InfoClient().Return(infoClient)

	return client
}

func chainRequest(backupID string) *model.RestoreRequest {
	return &model.RestoreRequest{
		SourceStorage:  &model.S3Storage{Bucket: "bucket", S3Region: "us-east-1"},
		BackupDataPath: backupID,
	}
}

func TestRestoreHandler_Wait_UntilNamespaceOpen(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	gomock.InOrder(
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(&infomodels.ResponseRestoreState{
			State:   infomodels.RestoreStateComplete,
			NsState: infomodels.NamespaceStateOpening,
		}, nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(&infomodels.ResponseRestoreState{
			State:   infomodels.RestoreStateComplete,
			NsState: infomodels.NamespaceStateNone,
		}, nil),
	)

	require.NoError(t, newRestoreHandler(infoClient, "bar", "job", laneOf(model.RestoreModeStrict)).Wait(t.Context()))
}

// TestRestoreHandler_Wait_ClearsNamespace checks that a cold restore whose nodes hold data in the
// namespace clears their storage, as each preparing node asks, and then waits for the restore.
func TestRestoreHandler_Wait_ClearsNamespace(t *testing.T) {
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
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(&infomodels.ResponseRestoreState{
			State: infomodels.RestoreStateComplete, NsState: infomodels.NamespaceStateNone,
		}, nil),
	)

	require.NoError(t, newRestoreHandler(infoClient, "bar", "job", laneOf(model.RestoreModeStrict)).Wait(t.Context()))
}

func TestRestoreHandler_Wait_ReleasesNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		expect  func(infoClient *aerospike.MockInfoGetter)
		wantErr string
	}{
		{
			name: "failed",
			expect: func(infoClient *aerospike.MockInfoGetter) {
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(
					&infomodels.ResponseRestoreState{State: infomodels.RestoreStateFailed}, nil)
			},
			wantErr: "server restore job failed",
		},
		{
			name: "aborted",
			expect: func(infoClient *aerospike.MockInfoGetter) {
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(
					&infomodels.ResponseRestoreState{State: infomodels.RestoreStateAborted}, nil)
			},
			wantErr: "server restore job was aborted",
		},
		{
			name: "status unavailable",
			expect: func(infoClient *aerospike.MockInfoGetter) {
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(nil, errors.New("timeout"))
			},
			wantErr: "failed to get server restore status: timeout",
		},
		{
			name: "clear refused",
			expect: func(infoClient *aerospike.MockInfoGetter) {
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(
					&infomodels.ResponseRestoreState{
						NsState: infomodels.NamespaceStatePreparing,
						Clear:   infomodels.ClearStateRequired,
					}, nil)
				infoClient.EXPECT().ClearRestoreStorage(gomock.Any(), "bar", "job").Return(errors.New("refused"))
			},
			wantErr: "failed to clear namespace bar for server restore job: refused",
		},
		{
			name: "held by truncate state",
			expect: func(infoClient *aerospike.MockInfoGetter) {
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(
					&infomodels.ResponseRestoreState{
						NsState:    infomodels.NamespaceStatePreparing,
						HeldReason: heldByTruncateState,
					}, nil)
			},
			wantErr: "clear it with the info command 'truncate-namespace-undo:namespace=bar'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			jobID := "job"
			infoClient := aerospike.NewMockInfoGetter(ctrl)
			tt.expect(infoClient)
			expectRelease(t, infoClient, &jobID)

			handler := newRestoreHandler(infoClient, "bar", jobID, laneOf(model.RestoreModeStrict))
			require.ErrorContains(t, handler.Wait(t.Context()), tt.wantErr)
		})
	}
}

func TestRestoreHandler_Wait_Canceled(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// The release still runs: the canceled restore holds the namespace.
	jobID := "job"
	infoClient := aerospike.NewMockInfoGetter(ctrl)
	expectRelease(t, infoClient, &jobID)

	handler := newRestoreHandler(infoClient, "bar", jobID, laneOf(model.RestoreModeStrict))
	require.ErrorIs(t, handler.Wait(ctx), context.Canceled)
}

func TestRestoreExecutor_Run_Fuzzy(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	// A fuzzy restore writes into the live namespace: no preparation.
	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *infomodels.RequestRestore) (string, error) {
			assert.Equal(t, infomodels.RestorePolicyFuzzy, request.Policy)
			assert.Equal(t, fullID+","+incr1ID, request.BackupIDs)

			return restoreJobID, nil
		})

	handler, err := NewRestoreExecutor(namespaceReader(t, ctrl)).Run(t.Context(), clientOf(ctrl, infoClient),
		fuzzyRequest(fullID), fuzzyRequest(incr1ID))
	require.NoError(t, err)
	assert.NotNil(t, handler)
}

func TestRestoreExecutor_Run_SkipsIndexesAndUDFs(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *infomodels.RequestRestore) (string, error) {
			assert.Equal(t, new(true), request.NoIndexes)
			assert.Equal(t, new(true), request.NoUDFs)

			return restoreJobID, nil
		})
	request := fuzzyRequest(fullID)
	request.Policy.NoIndexes = new(true)
	request.Policy.NoUdfs = new(true)

	_, err := NewRestoreExecutor(namespaceReader(t, ctrl)).Run(t.Context(), clientOf(ctrl, infoClient), request)
	require.NoError(t, err)
}

// TestRestoreHandler_Wait_FuzzyAbortsOnly checks that a fuzzy restore that does not complete is
// aborted, and that its namespace, which it never held, is not opened.
func TestRestoreHandler_Wait_FuzzyAbortsOnly(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(
		&infomodels.ResponseRestoreState{State: infomodels.RestoreStateFailed}, nil)
	infoClient.EXPECT().AbortRestore(gomock.Any(), "bar", "job").Return(nil)

	err := newRestoreHandler(infoClient, "bar", "job", laneOf(model.RestoreModeFuzzy)).Wait(t.Context())
	require.ErrorContains(t, err, "server restore job failed")
}

func fuzzyRequest(backupID string) *model.RestoreRequest {
	request := chainRequest(backupID)
	request.Policy.Mode = model.RestoreModeFuzzy

	return request
}
