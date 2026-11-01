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

			var preparedJobID string
			infoClient := aerospike.NewMockInfoGetter(ctrl)
			gomock.InOrder(
				infoClient.EXPECT().PrepareRestore(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, request *infomodels.RequestPrepareRestore) error {
						assert.Equal(t, "bar", request.Namespace)
						preparedJobID = request.JobID

						return nil
					}),
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", gomock.Any()).Return(
					&infomodels.ResponseRestoreState{NsState: infomodels.NamespaceStateReady}, nil),
				infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, request *infomodels.RequestRestore) error {
						assert.Equal(t, tt.wantBackupIDs, request.BackupIDs)
						assert.Equal(t, "bar", request.Namespace)
						assert.Equal(t, infomodels.RestorePolicyStrict, request.Policy)
						assert.Equal(t, preparedJobID, request.JobID, "the restore starts the prepared job")

						return nil
					}),
			)

			requests := make([]*model.RestoreRequest, 0, len(tt.backupIDs))
			for _, id := range tt.backupIDs {
				requests = append(requests, chainRequest(id))
			}

			handler, err := NewRestoreExecutor(namespaceReader(t, ctrl)).
				Run(t.Context(), clientOf(ctrl, infoClient), requests[0], requests[1:]...)
			require.NoError(t, err)
			assert.NotNil(t, handler)
		})
	}
}

func TestRestoreExecutor_Run_ClearsNamespace(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	gomock.InOrder(
		infoClient.EXPECT().PrepareRestore(gomock.Any(), gomock.Any()).Return(nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", gomock.Any()).Return(
			&infomodels.ResponseRestoreState{
				NsState: infomodels.NamespaceStatePreparing,
				Clear:   infomodels.ClearStateRequired,
			}, nil),
		infoClient.EXPECT().ClearRestoreStorage(gomock.Any(), "bar", gomock.Any()).Return(nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", gomock.Any()).Return(
			&infomodels.ResponseRestoreState{NsState: infomodels.NamespaceStateReady}, nil),
		infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).Return(nil),
	)

	_, err := NewRestoreExecutor(namespaceReader(t, ctrl)).Run(t.Context(), clientOf(ctrl, infoClient),
		chainRequest(fullID), chainRequest(incr1ID))
	require.NoError(t, err)
}

// TestRestoreExecutor_Run_ReleasesNamespace checks that a restore that fails after its
// preparation was accepted aborts the job and opens the namespace.
func TestRestoreExecutor_Run_ReleasesNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		expect  func(infoClient *aerospike.MockInfoGetter)
		wantErr string
	}{
		{
			name: "start refused",
			expect: func(infoClient *aerospike.MockInfoGetter) {
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", gomock.Any()).Return(
					&infomodels.ResponseRestoreState{NsState: infomodels.NamespaceStateReady}, nil)
				infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).Return(errors.New("refused"))
			},
			wantErr: "failed to start server restore",
		},
		{
			name: "preparation failed",
			expect: func(infoClient *aerospike.MockInfoGetter) {
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", gomock.Any()).Return(
					&infomodels.ResponseRestoreState{NsState: infomodels.NamespaceStateFailed}, nil)
			},
			wantErr: "failed to prepare namespace bar",
		},
		{
			name: "clear refused",
			expect: func(infoClient *aerospike.MockInfoGetter) {
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", gomock.Any()).Return(
					&infomodels.ResponseRestoreState{
						NsState: infomodels.NamespaceStatePreparing,
						Clear:   infomodels.ClearStateRequired,
					}, nil)
				infoClient.EXPECT().ClearRestoreStorage(gomock.Any(), "bar", gomock.Any()).Return(errors.New("refused"))
			},
			wantErr: "failed to clear namespace bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			var jobID string
			infoClient := aerospike.NewMockInfoGetter(ctrl)
			infoClient.EXPECT().PrepareRestore(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, request *infomodels.RequestPrepareRestore) error {
					jobID = request.JobID

					return nil
				})
			tt.expect(infoClient)
			expectRelease(t, infoClient, &jobID)

			_, err := NewRestoreExecutor(namespaceReader(t, ctrl)).Run(t.Context(), clientOf(ctrl, infoClient),
				chainRequest(fullID))
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestRestoreExecutor_Run_PrepareRefused(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	// A refused restore-prepare leaves the namespace open: nothing to release.
	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().PrepareRestore(gomock.Any(), gomock.Any()).Return(errors.New("refused"))

	_, err := NewRestoreExecutor(namespaceReader(t, ctrl)).Run(t.Context(), clientOf(ctrl, infoClient),
		chainRequest(fullID))
	require.ErrorContains(t, err, "failed to prepare server restore")
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

	require.NoError(t, newRestoreHandler(infoClient, "bar", "job").Wait(t.Context()))
}

func TestRestoreHandler_Wait_ReleasesNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		state   infomodels.RestoreState
		wantErr string
	}{
		{name: "failed", state: infomodels.RestoreStateFailed, wantErr: "server restore job failed"},
		{name: "aborted", state: infomodels.RestoreStateAborted, wantErr: "server restore job was aborted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			jobID := "job"
			infoClient := aerospike.NewMockInfoGetter(ctrl)
			infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", jobID).Return(
				&infomodels.ResponseRestoreState{State: tt.state}, nil)
			expectRelease(t, infoClient, &jobID)

			require.ErrorContains(t, newRestoreHandler(infoClient, "bar", jobID).Wait(t.Context()), tt.wantErr)
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

	require.ErrorIs(t, newRestoreHandler(infoClient, "bar", jobID).Wait(ctx), context.Canceled)
}
