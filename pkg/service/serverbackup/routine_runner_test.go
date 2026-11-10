package serverbackup

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go/pkg/asinfo"
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
			{BackupMetadata: model.BackupMetadata{Namespace: "qux", Created: at(20)}},
		},
		model.BackupTypeIncremental: {
			{BackupMetadata: model.BackupMetadata{Namespace: "qux", Created: at(5)}},
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
		{name: "incremental after a newer full backup starts from the full", backupType: model.BackupTypeIncremental,
			namespace: "qux", want: new(at(20))},
		{name: "cumulative after a newer full backup", backupType: model.BackupTypeIncremental,
			incrMode: model.IncrModeCumulative, namespace: "qux", want: new(at(20))},
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

func TestRoutineRunner_ModifiedAfter_ReadFails(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	backupReader := service.NewMockBackupReader(ctrl)
	backupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(nil, errors.New("storage down"))
	routine := &model.BackupRoutine{Name: "routine", BackupPolicy: &model.BackupPolicy{}}

	got, err := (&routineRunner{backupReader: backupReader}).
		modifiedAfter(t.Context(), routine, model.BackupTypeIncremental, "bar")

	require.EqualError(t, err, "failed to read the last backup of namespace bar: storage down")
	assert.Nil(t, got)
}

// expectCompleteJobs records each backup request, names its job job-<namespace> and reports the
// job complete.
func expectCompleteJobs(infoClient *aerospike.MockInfoGetter, requests *[]infomodels.RequestBackup) {
	infoClient.EXPECT().StartBackup(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *infomodels.RequestBackup) (string, error) {
			*requests = append(*requests, *request)

			return "job-" + request.Namespace, nil
		}).AnyTimes()
	infoClient.EXPECT().GetBackupStatus(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, jobID string) (*infomodels.ResponseBackupState, error) {
			return &infomodels.ResponseBackupState{JobID: jobID, State: infomodels.BackupStateComplete}, nil
		}).AnyTimes()
}

func TestRoutineRunner_BackupNamespaces_Requests(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	at := func(minutes int) time.Time { return start.Add(time.Duration(minutes) * time.Minute) }
	unix := func(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }

	storage := &model.S3Storage{Bucket: "bucket", S3Region: "eu-central-1", Path: "backups", S3Profile: "backup"}
	common := func(namespace string) infomodels.RequestCommon {
		return infomodels.RequestCommon{
			Namespace: namespace, Storage: "aws-s3", Path: "backups", Bucket: "bucket",
			Region: "eu-central-1", Profile: "backup",
		}
	}

	backups := map[model.BackupType][]model.BackupDetails{
		model.BackupTypeFull: {
			{BackupMetadata: model.BackupMetadata{Namespace: "bar", Created: at(0)}},
			{BackupMetadata: model.BackupMetadata{Namespace: "baz", Created: at(1)}},
		},
		model.BackupTypeIncremental: {
			{BackupMetadata: model.BackupMetadata{Namespace: "bar", Created: at(10)}},
		},
	}

	tests := []struct {
		name       string
		runSpec    model.BackupRunSpec
		namespaces []string
		want       []infomodels.RequestBackup
	}{
		{
			// A sealed backup is bounded by the time it started.
			name:       "full",
			runSpec:    model.BackupRunSpec{Type: model.BackupTypeFull, TimeBounds: model.TimeBounds{ToTime: new(at(30))}},
			namespaces: []string{"bar"},
			want:       []infomodels.RequestBackup{{RequestCommon: common("bar"), ModifiedBefore: unix(at(30))}},
		},
		{
			// Each namespace starts where its own last backup did; the run's lower bound, taken
			// from the routine's last run, is not used.
			name: "incremental of several namespaces",
			runSpec: model.BackupRunSpec{
				Type:       model.BackupTypeIncremental,
				TimeBounds: model.TimeBounds{FromTime: new(at(25))},
			},
			namespaces: []string{"bar", "baz"},
			want: []infomodels.RequestBackup{
				{RequestCommon: common("bar"), ModifiedAfter: unix(at(10))},
				{RequestCommon: common("baz"), ModifiedAfter: unix(at(1))},
			},
		},
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

			var requests []infomodels.RequestBackup
			infoClient := aerospike.NewMockInfoGetter(ctrl)
			expectCompleteJobs(infoClient, &requests)
			routine := &model.BackupRoutine{Name: "routine", Storage: storage, BackupPolicy: &model.BackupPolicy{}}

			err := (&routineRunner{backupReader: backupReader}).backupNamespaces(t.Context(), infoClient,
				routine, tt.runSpec, tt.namespaces, slog.New(slog.DiscardHandler))

			require.NoError(t, err)
			assert.Equal(t, tt.want, requests, "one server job per namespace, in the namespaces' order")
		})
	}
}

// TestRoutineRunner_BackupNamespaces_Failures checks that a namespace that fails does not stop the
// ones after it, and that the run fails with the error of each namespace that failed.
func TestRoutineRunner_BackupNamespaces_Failures(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	gomock.InOrder(
		infoClient.EXPECT().StartBackup(gomock.Any(), gomock.Any()).Return("", errors.New("refused")),
		infoClient.EXPECT().StartBackup(gomock.Any(), gomock.Any()).Return("job-baz", nil),
		infoClient.EXPECT().GetBackupStatus(gomock.Any(), "job-baz").Return(&infomodels.ResponseBackupState{
			JobID: "job-baz", Namespace: "baz", State: infomodels.BackupStateFailed, ErrorReason: "disk full",
		}, nil),
		infoClient.EXPECT().StartBackup(gomock.Any(), gomock.Any()).Return("job-qux", nil),
		infoClient.EXPECT().GetBackupStatus(gomock.Any(), "job-qux").Return(&infomodels.ResponseBackupState{
			JobID: "job-qux", Namespace: "qux", State: infomodels.BackupStateComplete,
		}, nil),
	)
	routine := &model.BackupRoutine{
		Storage:      &model.S3Storage{Bucket: "bucket", S3Region: "us-east-1"},
		BackupPolicy: &model.BackupPolicy{},
	}

	err := (&routineRunner{}).backupNamespaces(t.Context(), infoClient, routine,
		model.BackupRunSpec{Type: model.BackupTypeFull}, []string{"bar", "baz", "qux"}, slog.New(slog.DiscardHandler))

	require.EqualError(t, err, "namespace bar: failed to start server backup: refused\n"+
		"namespace baz: server backup job-baz failed: disk full")
}

func TestRoutineRunner_BackupNamespaces_NotS3(t *testing.T) {
	t.Parallel()

	// Nothing is sent to the server.
	infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
	routine := &model.BackupRoutine{Storage: &model.LocalStorage{Path: "/backups"}, BackupPolicy: &model.BackupPolicy{}}

	err := (&routineRunner{}).backupNamespaces(t.Context(), infoClient, routine,
		model.BackupRunSpec{Type: model.BackupTypeFull}, []string{"bar"}, slog.New(slog.DiscardHandler))

	require.EqualError(t, err, "namespace bar: server backup requires S3 storage, got *model.LocalStorage")
}

func TestWaitForBackup_Outcome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		final   *infomodels.ResponseBackupState
		wantErr string
	}{
		{name: "complete", final: &infomodels.ResponseBackupState{State: infomodels.BackupStateComplete}},
		{
			name:    "failed",
			final:   &infomodels.ResponseBackupState{State: infomodels.BackupStateFailed, ErrorReason: "s3 PUT 403"},
			wantErr: "server backup " + fullID + " failed: s3 PUT 403",
		},
		{
			name:    "aborted on the server",
			final:   &infomodels.ResponseBackupState{State: infomodels.BackupStateAborted},
			wantErr: "server backup " + fullID + " was aborted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
			gomock.InOrder(
				infoClient.EXPECT().GetBackupStatus(gomock.Any(), fullID).Return(
					&infomodels.ResponseBackupState{State: infomodels.BackupStateBaseScanActive}, nil),
				infoClient.EXPECT().GetBackupStatus(gomock.Any(), fullID).Return(tt.final, nil),
			)

			err := waitForBackup(t.Context(), infoClient, fullID, slog.New(slog.DiscardHandler))

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestWaitForBackup_StatusLookup(t *testing.T) {
	t.Parallel()

	complete := &infomodels.ResponseBackupState{State: infomodels.BackupStateComplete}

	tests := []struct {
		name      string
		statuses  []func() (*infomodels.ResponseBackupState, error)
		wantAbort bool
		wantErr   string
	}{
		{
			name: "job not visible yet",
			statuses: []func() (*infomodels.ResponseBackupState, error){
				func() (*infomodels.ResponseBackupState, error) { return nil, asinfo.ErrNotFound },
				func() (*infomodels.ResponseBackupState, error) { return complete, nil },
			},
		},
		{
			// The run gives up on the job, so it aborts it: the job may still hold the cluster.
			name: "status unavailable",
			statuses: []func() (*infomodels.ResponseBackupState, error){
				func() (*infomodels.ResponseBackupState, error) { return nil, errors.New("timeout") },
			},
			wantAbort: true,
			wantErr:   "failed to get server backup status: timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
			calls := make([]any, 0, len(tt.statuses)+1)
			for _, status := range tt.statuses {
				calls = append(calls, infoClient.EXPECT().GetBackupStatus(gomock.Any(), fullID).Return(status()))
			}
			if tt.wantAbort {
				calls = append(calls, infoClient.EXPECT().AbortBackup(gomock.Any(), fullID).Return(nil))
			}
			gomock.InOrder(calls...)

			err := waitForBackup(t.Context(), infoClient, fullID, slog.New(slog.DiscardHandler))

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

// TestWaitForBackup_Canceled checks that a canceled run aborts its job on a live context, and
// that a refused abort leaves the run's error as the cancellation.
func TestWaitForBackup_Canceled(t *testing.T) {
	t.Parallel()

	for _, abortErr := range []error{nil, errors.New("not principal")} {
		infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
		infoClient.EXPECT().AbortBackup(gomock.Any(), fullID).DoAndReturn(func(ctx context.Context, _ string) error {
			require.NoError(t, ctx.Err(), "the abort runs on a live context")

			return abortErr
		})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := waitForBackup(ctx, infoClient, fullID, slog.New(slog.DiscardHandler))

		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestBackupOperation_Status(t *testing.T) {
	t.Parallel()

	operation := newBackupOperation(func() {})

	// The server counts no records, so there is no scan slot.
	run := operation.Status()
	assert.Equal(t, operation.startTime, run.StartTime)
	assert.Nil(t, run.Scan)
}
