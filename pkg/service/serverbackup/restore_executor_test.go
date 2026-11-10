package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var discardLogger = slog.New(slog.DiscardHandler)

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

func TestRestoreExecutor_Run_Strict(t *testing.T) {
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

			// The restore starts with one command and then follows the job that command returned.
			infoClient := aerospike.NewMockInfoGetter(ctrl)
			gomock.InOrder(
				infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, request *infomodels.RequestRestore) (string, error) {
						assert.Equal(t, tt.wantBackupIDs, request.BackupIDs)
						assert.Equal(t, "bar", request.Namespace)
						assert.Equal(t, "backups/routine", request.Path, "the routine's folder in the storage")
						assert.Equal(t, infomodels.RestorePolicyStrict, request.Policy)

						return restoreJobID, nil
					}),
				infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", restoreJobID).Return(completeStatus, nil),
			)

			requests := make([]*model.RestoreRequest, 0, len(tt.backupIDs))
			for _, id := range tt.backupIDs {
				requests = append(requests, strictRequest(id))
			}

			handler := NewRestoreExecutor(namespaceReader(t, ctrl)).
				Run(t.Context(), clientOf(ctrl, infoClient), discardLogger, requests...)
			require.NoError(t, handler.Wait(t.Context()))
		})
	}
}

func TestRestoreExecutor_Run_Fuzzy(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *infomodels.RequestRestore) (string, error) {
			assert.Equal(t, infomodels.RestorePolicyFuzzy, request.Policy)
			assert.Equal(t, fullID+","+incr1ID, request.BackupIDs)

			return restoreJobID, nil
		})
	infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", restoreJobID).Return(completeStatus, nil)

	handler := NewRestoreExecutor(namespaceReader(t, ctrl)).Run(t.Context(), clientOf(ctrl, infoClient),
		discardLogger, fuzzyRequest(fullID), fuzzyRequest(incr1ID))
	require.NoError(t, handler.Wait(t.Context()))
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
	infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", restoreJobID).Return(completeStatus, nil)
	request := fuzzyRequest(fullID)
	request.Policy.NoIndexes = new(true)
	request.Policy.NoUdfs = new(true)

	handler := NewRestoreExecutor(namespaceReader(t, ctrl)).Run(t.Context(), clientOf(ctrl, infoClient),
		discardLogger, request)
	require.NoError(t, handler.Wait(t.Context()))
}

// TestRestoreExecutor_Run_Retry checks that the server gets the retry settings that the policy of
// a fuzzy restore sets, and no others.
func TestRestoreExecutor_Run_Retry(t *testing.T) {
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

			request := fuzzyRequest(fullID)
			request.Policy.RetryPolicy = tt.policy
			handler := NewRestoreExecutor(namespaceReader(t, ctrl)).
				Run(t.Context(), clientOf(ctrl, infoClient), discardLogger, request)
			require.NoError(t, handler.Wait(t.Context()))
		})
	}
}

// TestRestoreExecutor_Run_StartRefused checks that a refused start fails the restore, explains a
// refusal for truncate state, and cleans nothing up, because a refused job never started.
func TestRestoreExecutor_Run_StartRefused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request *model.RestoreRequest
		refusal string
		wantErr string
	}{
		{name: "strict", request: strictRequest(fullID), refusal: "refused",
			wantErr: "failed to start server restore: refused"},
		{name: "fuzzy", request: fuzzyRequest(fullID), refusal: "refused",
			wantErr: "failed to start server restore: refused"},
		{
			name:    "truncate state",
			request: strictRequest(fullID),
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

			err := NewRestoreExecutor(namespaceReader(t, ctrl)).
				Run(t.Context(), clientOf(ctrl, infoClient), discardLogger, tt.request).Wait(t.Context())
			require.ErrorContains(t, err, tt.wantErr)
			require.ErrorContains(t, err, tt.refusal)
		})
	}
}

// TestRestoreExecutor_Run_StartOutcomeUnknown checks that a start whose outcome is unknown
// releases the job it may have started: a strict job is aborted and its namespace opened, a fuzzy
// job is only aborted.
func TestRestoreExecutor_Run_StartOutcomeUnknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request *model.RestoreRequest
		expect  func(infoClient *aerospike.MockInfoGetter)
	}{
		{name: "strict", request: strictRequest(fullID), expect: func(infoClient *aerospike.MockInfoGetter) {
			expectRelease(infoClient, restoreJobID)
		}},
		{name: "fuzzy", request: fuzzyRequest(fullID), expect: func(infoClient *aerospike.MockInfoGetter) {
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

			err := NewRestoreExecutor(namespaceReader(t, ctrl)).
				Run(t.Context(), clientOf(ctrl, infoClient), discardLogger, tt.request).Wait(t.Context())
			require.ErrorIs(t, err, asinfo.ErrOutcomeUnknown)
		})
	}
}

// TestRestoreExecutor_Run_InvalidBackupKey checks that a run that fails before it reaches the
// server returns its failure from the handler's Wait.
func TestRestoreExecutor_Run_InvalidBackupKey(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	request := strictRequest(fullID)
	request.BackupDataPath = fullID // a bare backup id names no routine.

	err := NewRestoreExecutor(service.NewMockBackupReader(ctrl)).
		Run(t.Context(), aerospike.NewMockClient(ctrl), discardLogger, request).Wait(t.Context())
	require.EqualError(t, err, `invalid server backup key "`+fullID+`": want <routine>/<job id>`)
}

func TestRestoreExecutor_Run_ChainTooLong(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	requests := make([]*model.RestoreRequest, maxChainLength+1)
	for i := range requests {
		requests[i] = strictRequest(fmt.Sprintf("261020T09%04d-aaaa", i))
	}

	err := NewRestoreExecutor(service.NewMockBackupReader(ctrl)).
		Run(t.Context(), aerospike.NewMockClient(ctrl), discardLogger, requests...).Wait(t.Context())
	require.EqualError(t, err, "restore chain has 33 backups, the server restores at most 32 in one job")
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

// strictRequest returns a strict restore request for one backup of the routine.
func strictRequest(backupID string) *model.RestoreRequest {
	return &model.RestoreRequest{
		Policy:         model.RestorePolicy{Mode: model.RestoreModeStrict},
		SourceStorage:  &model.S3Storage{Bucket: "bucket", S3Region: "us-east-1", Path: "backups"},
		BackupDataPath: backupKey("routine", backupID),
	}
}

// fuzzyRequest returns a fuzzy restore request for one backup of the routine.
func fuzzyRequest(backupID string) *model.RestoreRequest {
	request := strictRequest(backupID)
	request.Policy.Mode = model.RestoreModeFuzzy

	return request
}
