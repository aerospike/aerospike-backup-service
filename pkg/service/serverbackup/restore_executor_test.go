package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go/pkg/asinfo"
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
						assert.Equal(t, "backups/routine", request.Path, "the routine's folder in the storage")
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

// TestRestoreExecutor_Run_StartFails checks that a start the server refused aborts nothing, and
// that a start whose outcome is unknown aborts the job it may have started, reporting a failed
// abort: that job may still be writing into the namespace.
func TestRestoreExecutor_Run_StartFails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		startErr error
		abortErr error
		wantErr  []string
	}{
		{name: "refused", startErr: errors.New("refused"), wantErr: []string{"failed to start server restore: refused"}},
		{name: "outcome unknown", startErr: fmt.Errorf("%w: timeout", asinfo.ErrOutcomeUnknown),
			wantErr: []string{"failed to start server restore"}},
		{name: "outcome unknown, abort fails", startErr: fmt.Errorf("%w: timeout", asinfo.ErrOutcomeUnknown),
			abortErr: errors.New("not principal"),
			wantErr: []string{"failed to start server restore",
				"failed to abort server restore " + restoreJobID + ": not principal"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			infoClient := aerospike.NewMockInfoGetter(ctrl)
			infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).Return(restoreJobID, tt.startErr)
			if errors.Is(tt.startErr, asinfo.ErrOutcomeUnknown) {
				infoClient.EXPECT().AbortRestore(gomock.Any(), "bar", restoreJobID).Return(tt.abortErr)
			}

			err := NewRestoreExecutor(namespaceReader(t, ctrl)).
				Run(t.Context(), clientOf(ctrl, infoClient), discardLogger, chainRequest(fullID)).Wait(t.Context())

			require.ErrorIs(t, err, tt.startErr)
			for _, want := range tt.wantErr {
				require.ErrorContains(t, err, want)
			}
		})
	}
}

// TestRestoreExecutor_Start_NamespaceRemap checks that a restore into another namespace is refused
// before anything is sent to the server, which can only write into the backup's own namespace.
func TestRestoreExecutor_Start_NamespaceRemap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace *model.RestoreNamespace
		wantErr   string
	}{
		{name: "into another namespace", namespace: &model.RestoreNamespace{Source: "bar", Destination: "baz"},
			wantErr: "server restore cannot restore namespace bar into baz"},
		{name: "into the same namespace", namespace: &model.RestoreNamespace{Source: "bar", Destination: "bar"}},
		{name: "no destination", namespace: &model.RestoreNamespace{Source: "bar"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			// start runs on the test goroutine, so a start the refusal should have prevented fails
			// the test rather than leaving a restore run blocked.
			client := aerospike.NewMockClient(ctrl)
			if tt.wantErr == "" {
				infoClient := aerospike.NewMockInfoGetter(ctrl)
				client.EXPECT().InfoClient().Return(infoClient)
				infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).Return(restoreJobID, nil)
			}
			request := chainRequest(fullID)
			request.Policy.Namespace = tt.namespace

			executor := &restoreExecutor{backupReader: namespaceReader(t, ctrl)}
			_, err := executor.start(t.Context(), client, []*model.RestoreRequest{request})

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestRestoreExecutor_Run_InvalidKey(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	request := chainRequest(fullID)
	request.BackupDataPath = fullID // a bare job id names no routine.

	err := NewRestoreExecutor(service.NewMockBackupReader(ctrl)).
		Run(t.Context(), aerospike.NewMockClient(ctrl), discardLogger, request).Wait(t.Context())
	require.EqualError(t, err, `invalid server backup key "`+fullID+`": want <routine>/<job id>`)
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
			assert.Equal(t, backupKey("routine", fullID), filter.(*service.PathFilter).Path(),
				"the namespace comes from the full backup")

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
		SourceStorage:  &model.S3Storage{Bucket: "bucket", S3Region: "us-east-1", Path: "backups"},
		BackupDataPath: backupKey("routine", backupID),
	}
}

// TestRestoreJob_Wait_LostAfterSeen checks that a job that a node stops knowing once it has been
// seen (a node that joined or restarted) is aborted, rather than polled until the run ends.
func TestRestoreJob_Wait_LostAfterSeen(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	gomock.InOrder(
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(nil, asinfo.ErrNotFound),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(
			&infomodels.ResponseRestoreState{State: infomodels.RestoreStateActive}, nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", "job").Return(nil, asinfo.ErrNotFound),
		infoClient.EXPECT().AbortRestore(gomock.Any(), "bar", "job").Return(nil),
	)

	err := (&restoreJob{infoClient: infoClient, namespace: "bar", jobID: "job"}).Wait(t.Context())
	require.ErrorIs(t, err, asinfo.ErrNotFound)
}
