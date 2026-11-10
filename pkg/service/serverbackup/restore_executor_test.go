package serverbackup

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var discardLogger = slog.New(slog.DiscardHandler)

const (
	fullID  = "261020T090111-aaaa"
	incr1ID = "261020T091111-bbbb"
	incr2ID = "261020T092111-cccc"
	// restoreJobID is the id the server restore job is started under.
	restoreJobID = "261105T094929-k9s5"
)

func TestRestoreExecutor_Run(t *testing.T) {
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

			infoClient := aerospike.NewMockInfoGetter(ctrl)
			gomock.InOrder(
				infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, request *infomodels.RequestRestore) (string, error) {
						assert.Equal(t, tt.wantBackupIDs, request.BackupIDs)
						assert.Equal(t, "bar", request.Namespace)
						assert.Equal(t, infomodels.RestorePolicyFuzzy, request.Policy)

						return restoreJobID, nil
					}),
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", restoreJobID).Return(
					&infomodels.ResponseRestoreState{State: infomodels.RestoreStateActive}, nil),
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", restoreJobID).Return(
					&infomodels.ResponseRestoreState{State: infomodels.RestoreStateComplete}, nil),
			)

			requests := make([]*model.RestoreRequest, 0, len(tt.backupIDs))
			for _, id := range tt.backupIDs {
				requests = append(requests, chainRequest(id))
			}

			handler := NewRestoreExecutor(namespaceReader(t, ctrl)).
				Run(t.Context(), clientOf(ctrl, infoClient), discardLogger, requests...)
			require.NoError(t, handler.Wait(t.Context()))
		})
	}
}

// TestRestoreExecutor_Run_StartRefused checks that a refused start fails the restore and aborts
// nothing: the refused job never ran.
func TestRestoreExecutor_Run_StartRefused(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).Return("", errors.New("refused"))

	err := NewRestoreExecutor(namespaceReader(t, ctrl)).
		Run(t.Context(), clientOf(ctrl, infoClient), discardLogger, chainRequest(fullID)).Wait(t.Context())
	require.ErrorContains(t, err, "failed to start server restore: refused")
}

// TestRestoreJob_Wait_AbortsUnfinished checks that a restore job that does not complete is aborted.
func TestRestoreJob_Wait_AbortsUnfinished(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  *infomodels.ResponseRestoreState
		err     error
		wantErr string
	}{
		{name: "failed", status: &infomodels.ResponseRestoreState{State: infomodels.RestoreStateFailed},
			wantErr: "server restore job failed"},
		{name: "aborted", status: &infomodels.ResponseRestoreState{State: infomodels.RestoreStateAborted},
			wantErr: "server restore job was aborted"},
		{name: "status unavailable", err: errors.New("timeout"),
			wantErr: "failed to get server restore status: timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			infoClient := aerospike.NewMockInfoGetter(ctrl)
			gomock.InOrder(
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(tt.status, tt.err),
				infoClient.EXPECT().AbortRestore(gomock.Any(), "bar", "job").Return(nil),
			)

			job := &restoreJob{infoClient: infoClient, namespace: "bar", jobID: "job"}
			require.ErrorContains(t, job.Wait(t.Context()), tt.wantErr)
		})
	}
}

// TestRestoreJob_Wait_Canceled checks that a canceled restore is aborted, though its context has
// ended, and that a refused abort is reported along with the cancellation.
func TestRestoreJob_Wait_Canceled(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().AbortRestore(gomock.Any(), "bar", "job").DoAndReturn(
		func(ctx context.Context, _, _ string) error {
			require.NoError(t, ctx.Err(), "the abort runs on a live context")

			return errors.New("refused")
		})

	err := (&restoreJob{infoClient: infoClient, namespace: "bar", jobID: "job"}).Wait(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, "failed to abort server restore job: refused")
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
