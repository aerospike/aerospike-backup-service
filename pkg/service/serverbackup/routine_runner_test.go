package serverbackup

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRoutineRunner_ModifiedAfter(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	at := func(minutes int) time.Time { return start.Add(time.Duration(minutes) * time.Minute) }

	// The namespaces are backed up one after another: baz's backups are newer than bar's.
	backups := map[model.BackupType][]model.BackupDetails{
		model.BackupTypeFull: {
			{BackupMetadata: model.BackupMetadata{Namespace: "bar", Created: at(0)}},
			{BackupMetadata: model.BackupMetadata{Namespace: "baz", Created: at(1)}},
		},
		model.BackupTypeIncremental: {
			{BackupMetadata: model.BackupMetadata{Namespace: "bar", Created: at(10)}},
			{BackupMetadata: model.BackupMetadata{Namespace: "baz", Created: at(11)}},
		},
	}

	tests := []struct {
		name       string
		backupType model.BackupType
		incrMode   model.IncrMode
		namespace  string
		want       *time.Time
	}{
		{name: "incremental from the namespace's last backup", backupType: model.BackupTypeIncremental,
			namespace: "bar", want: new(at(10))},
		{name: "cumulative from the namespace's last full backup", backupType: model.BackupTypeIncremental,
			incrMode: model.IncrModeCumulative, namespace: "bar", want: new(at(0))},
		{name: "incremental of a namespace with no backup", backupType: model.BackupTypeIncremental,
			namespace: "foo"},
		{name: "full has no lower bound", backupType: model.BackupTypeFull, namespace: "bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			backupReader := service.NewMockBackupReader(ctrl)
			backupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, filter service.BackupFilter) ([]model.BackupDetails, error) {
					return backups[filter.(*service.RoutineFilter).BackupType()], nil
				}).AnyTimes()

			routine := &model.BackupRoutine{
				Name:         "routine",
				BackupPolicy: &model.BackupPolicy{IncrMode: tt.incrMode},
			}

			got, err := (&routineRunner{backupReader: backupReader}).
				modifiedAfter(t.Context(), routine, tt.backupType, tt.namespace)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWaitForBackup_RecordsServerJob(t *testing.T) {
	t.Parallel()

	running := &infomodels.ResponseBackupState{
		JobID: fullID, Namespace: "test", State: infomodels.BackupStateBaseScanActive,
	}

	tests := []struct {
		name    string
		final   *infomodels.ResponseBackupState
		wantErr string
		wantJob model.ServerBackupJob
	}{
		{
			name: "complete",
			final: &infomodels.ResponseBackupState{
				JobID: fullID, Namespace: "test", State: infomodels.BackupStateComplete,
				RecsBackedUp: 10, BytesBackedUp: 100, RecsReadFailed: 1,
			},
			wantJob: model.ServerBackupJob{
				JobID: fullID, Namespace: "test", State: "COMPLETE",
				RecordsBackedUp: 10, BytesBackedUp: 100, RecordsReadFailed: 1,
			},
		},
		{
			name: "failed",
			final: &infomodels.ResponseBackupState{
				JobID: fullID, Namespace: "test", State: infomodels.BackupStateFailed, ErrorReason: "s3 PUT 403",
			},
			wantErr: "failed: s3 PUT 403",
			wantJob: model.ServerBackupJob{JobID: fullID, Namespace: "test", State: "FAILED", ErrorReason: "s3 PUT 403"},
		},
		{
			name: "aborted on the server",
			final: &infomodels.ResponseBackupState{
				JobID: fullID, Namespace: "test", State: infomodels.BackupStateAborted,
				ErrorReason: "node left", AbortOrigin: infomodels.AbortOriginCluster,
			},
			wantErr: "was aborted",
			wantJob: model.ServerBackupJob{
				JobID: fullID, Namespace: "test", State: "ABORTED", ErrorReason: "node left", AbortOrigin: "cluster",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
			gomock.InOrder(
				infoClient.EXPECT().GetBackupStatus(gomock.Any(), fullID).Return(running, nil),
				infoClient.EXPECT().GetBackupStatus(gomock.Any(), fullID).Return(tt.final, nil),
			)
			operation := newBackupOperation(func() {})

			err := waitForBackup(t.Context(), infoClient, fullID, operation, slog.New(slog.DiscardHandler))

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
			assert.Equal(t, []model.ServerBackupJob{tt.wantJob}, operation.ServerJobs())
		})
	}
}

func TestWaitForBackup_Canceled_RecordsAbortedJob(t *testing.T) {
	t.Parallel()
	infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
	// The server reports the abort origin once the abort has ended the job.
	gomock.InOrder(
		infoClient.EXPECT().AbortBackup(gomock.Any(), fullID).Return(nil),
		infoClient.EXPECT().GetBackupStatus(gomock.Any(), fullID).Return(&infomodels.ResponseBackupState{
			JobID: fullID, Namespace: "test", State: infomodels.BackupStateAborting,
		}, nil),
		infoClient.EXPECT().GetBackupStatus(gomock.Any(), fullID).Return(&infomodels.ResponseBackupState{
			JobID: fullID, Namespace: "test", State: infomodels.BackupStateAborted, AbortOrigin: infomodels.AbortOriginOperator,
		}, nil),
	)
	operation := newBackupOperation(func() {})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := waitForBackup(ctx, infoClient, fullID, operation, slog.New(slog.DiscardHandler))

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []model.ServerBackupJob{
		{JobID: fullID, Namespace: "test", State: "ABORTED", AbortOrigin: "operator"},
	}, operation.ServerJobs())
}

func TestBackupOperation_ServerJobs_OnePerJob(t *testing.T) {
	t.Parallel()
	operation := newBackupOperation(func() {})

	status := func(jobID, namespace string, state infomodels.BackupState) *infomodels.ResponseBackupState {
		return &infomodels.ResponseBackupState{JobID: jobID, Namespace: namespace, State: state}
	}

	operation.report(status(fullID, "bar", infomodels.BackupStateInit))
	operation.report(status(fullID, "bar", infomodels.BackupStateComplete))
	operation.report(status(incr1ID, "baz", infomodels.BackupStateFailed))

	assert.Equal(t, []model.ServerBackupJob{
		{JobID: fullID, Namespace: "bar", State: "COMPLETE"},
		{JobID: incr1ID, Namespace: "baz", State: "FAILED"},
	}, operation.ServerJobs())
}
