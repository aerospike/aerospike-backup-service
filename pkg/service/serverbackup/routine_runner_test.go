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
		routine    *model.BackupRoutine
		runSpec    model.BackupRunSpec
		namespaces []string
		want       []infomodels.RequestBackup
	}{
		{
			name: "full with every option",
			routine: &model.BackupRoutine{
				Storage:          storage,
				SetList:          []string{"setA", "setB"},
				BinList:          []string{"bin1", "bin2"},
				FilterExpression: "kxkCkwECAA==",
				BackupPolicy: &model.BackupPolicy{
					RecordsPerSecond: new(100),
					Parallel:         new(2),
					ParallelWrite:    new(3),
					NoIndexes:        new(true),
					NoUdfs:           new(false),
				},
			},
			// A sealed backup is bounded by the time it started.
			runSpec:    model.BackupRunSpec{Type: model.BackupTypeFull, TimeBounds: model.TimeBounds{ToTime: new(at(30))}},
			namespaces: []string{"bar"},
			want: []infomodels.RequestBackup{{
				RequestCommon: func() infomodels.RequestCommon {
					c := common("bar")
					c.SetList = "setA,setB"
					c.FilterExp = "kxkCkwECAA=="
					c.NoIndexes = new(true)
					c.NoUDFs = new(false)

					return c
				}(),
				BinList:             "bin1,bin2",
				ModifiedBefore:      unix(at(30)),
				BackupRPS:           100,
				BackupWorkerThreads: 2,
				BackupUploadThreads: 3,
			}},
		},
		{
			// Each namespace starts where its own last backup did; the run's lower bound, taken
			// from the routine's last run, is not used. An unset limit is left to the server.
			name:    "incremental of several namespaces",
			routine: &model.BackupRoutine{Name: "routine", Storage: storage, BackupPolicy: &model.BackupPolicy{}},
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
			operation := newBackupOperation(func() {})

			err := (&routineRunner{backupReader: backupReader}).backupNamespaces(t.Context(), infoClient,
				tt.routine, tt.runSpec, tt.namespaces, operation, slog.New(slog.DiscardHandler))

			require.NoError(t, err)
			assert.Equal(t, tt.want, requests)
			jobIDs := make([]string, 0, len(tt.namespaces))
			for _, job := range operation.Status().Server.Jobs {
				jobIDs = append(jobIDs, job.JobID)
			}
			assert.Len(t, jobIDs, len(tt.namespaces), "one server job per namespace")
			for i, namespace := range tt.namespaces {
				assert.Equal(t, "job-"+namespace, jobIDs[i], "the jobs run in the namespaces' order")
			}
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
	operation := newBackupOperation(func() {})

	err := (&routineRunner{}).backupNamespaces(t.Context(), infoClient, routine,
		model.BackupRunSpec{Type: model.BackupTypeFull}, []string{"bar", "baz", "qux"}, operation,
		slog.New(slog.DiscardHandler))

	require.EqualError(t, err, "namespace bar: failed to start server backup: refused\n"+
		"namespace baz: server backup job-baz failed: disk full")
	assert.Equal(t, []model.ServerBackupJob{
		{JobID: "job-baz", Namespace: "baz", State: "FAILED", ErrorReason: "disk full"},
		{JobID: "job-qux", Namespace: "qux", State: "COMPLETE"},
	}, operation.Status().Server.Jobs)
}

func TestRoutineRunner_BackupNamespaces_NotS3(t *testing.T) {
	t.Parallel()

	// Nothing is sent to the server.
	infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
	routine := &model.BackupRoutine{Storage: &model.LocalStorage{Path: "/backups"}, BackupPolicy: &model.BackupPolicy{}}

	err := (&routineRunner{}).backupNamespaces(t.Context(), infoClient, routine,
		model.BackupRunSpec{Type: model.BackupTypeFull}, []string{"bar"}, newBackupOperation(func() {}),
		slog.New(slog.DiscardHandler))

	require.EqualError(t, err, "namespace bar: server backup requires S3 storage, got *model.LocalStorage")
}

func TestWaitForBackup_RecordsServerJob(t *testing.T) {
	t.Parallel()

	jobStart := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	jobFinish := jobStart.Add(time.Minute)
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
				StartTime: jobStart, FinishTime: jobFinish,
			},
			wantJob: model.ServerBackupJob{
				JobID: fullID, Namespace: "test", State: "COMPLETE",
				RecordsBackedUp: 10, BytesBackedUp: 100, RecordsReadFailed: 1,
				StartTime: &jobStart, FinishTime: &jobFinish,
			},
		},
		{
			name: "failed",
			// A job that fails before it starts has no times.
			final: &infomodels.ResponseBackupState{
				JobID: fullID, Namespace: "test", State: infomodels.BackupStateFailed, ErrorReason: "s3 PUT 403",
			},
			wantErr: "server backup " + fullID + " failed: s3 PUT 403",
			wantJob: model.ServerBackupJob{JobID: fullID, Namespace: "test", State: "FAILED", ErrorReason: "s3 PUT 403"},
		},
		{
			name: "aborted on the server",
			final: &infomodels.ResponseBackupState{
				JobID: fullID, Namespace: "test", State: infomodels.BackupStateAborted,
				ErrorReason: "node left", AbortOrigin: infomodels.AbortOriginCluster,
			},
			wantErr: "server backup " + fullID + " was aborted",
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
			assert.Equal(t, []model.ServerBackupJob{tt.wantJob}, operation.Status().Server.Jobs)
		})
	}
}

func TestWaitForBackup_StatusLookup(t *testing.T) {
	t.Parallel()

	complete := &infomodels.ResponseBackupState{JobID: fullID, Namespace: "test", State: infomodels.BackupStateComplete}

	tests := []struct {
		name      string
		statuses  []func() (*infomodels.ResponseBackupState, error)
		wantAbort bool
		wantErr   string
		wantJobs  []model.ServerBackupJob
	}{
		{
			name: "job not visible yet",
			statuses: []func() (*infomodels.ResponseBackupState, error){
				func() (*infomodels.ResponseBackupState, error) { return nil, asinfo.ErrNotFound },
				func() (*infomodels.ResponseBackupState, error) { return complete, nil },
			},
			wantJobs: []model.ServerBackupJob{{JobID: fullID, Namespace: "test", State: "COMPLETE"}},
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
				// A refused abort is not followed: there is nothing to wait for.
				calls = append(calls, infoClient.EXPECT().AbortBackup(gomock.Any(), fullID).Return(errors.New("timeout")))
			}
			gomock.InOrder(calls...)
			operation := newBackupOperation(func() {})

			err := waitForBackup(t.Context(), infoClient, fullID, operation, slog.New(slog.DiscardHandler))

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErr)
			}
			assert.Equal(t, tt.wantJobs, operation.Status().Server.Jobs)
		})
	}
}

func TestWaitForBackup_Canceled_AbortRefused(t *testing.T) {
	t.Parallel()

	// The job's status is not looked up: the abort did not reach it.
	infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
	infoClient.EXPECT().AbortBackup(gomock.Any(), fullID).Return(errors.New("not principal"))
	operation := newBackupOperation(func() {})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := waitForBackup(ctx, infoClient, fullID, operation, slog.New(slog.DiscardHandler))

	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, operation.Status().Server.Jobs)
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
	}, operation.Status().Server.Jobs)
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
	}, operation.Status().Server.Jobs)
}

func TestBackupOperation_ReportsProgress(t *testing.T) {
	t.Parallel()
	operation := newBackupOperation(func() {})

	// The server reports percent, the operation a fraction, and no scan slot: it counts no records.
	for _, percent := range []float64{0, 42.7, 95, 100} {
		operation.report(&infomodels.ResponseBackupState{JobID: fullID, ProgressPct: percent})

		run := operation.Status()
		assert.InDelta(t, percent/100, run.Progress, 1e-9, "progress %v", percent)
		assert.Nil(t, run.Scan)
	}
}

// shortenLostJobTimeout shortens lostJobTimeout for the test. The test must not be parallel:
// parallel tests start only once the sequential ones have ended, so none reads it meanwhile.
func shortenLostJobTimeout(t *testing.T) {
	t.Helper()

	previous := lostJobTimeout
	lostJobTimeout = 1500 * time.Millisecond
	t.Cleanup(func() { lostJobTimeout = previous })
}

//nolint:paralleltest // shortens lostJobTimeout, which the parallel tests read.
func TestWaitForBackup_LostJob(t *testing.T) {
	shortenLostJobTimeout(t)

	tests := []struct {
		name     string
		statuses []*infomodels.ResponseBackupState
		wantErr  string
	}{
		{
			// Each status resets the clock: two not-found answers a status apart are within it.
			name: "seen again in time",
			statuses: []*infomodels.ResponseBackupState{
				nil,
				{JobID: fullID, State: infomodels.BackupStateBaseScanActive},
				nil,
				{JobID: fullID, State: infomodels.BackupStateComplete},
			},
		},
		{
			// The run aborts the job it gave up on; a lost job refuses the abort.
			name:     "never reported",
			statuses: []*infomodels.ResponseBackupState{nil, nil},
			wantErr:  "server backup " + fullID + " not reported for 1.5s: " + asinfo.ErrNotFound.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
			calls := make([]any, 0, len(tt.statuses)+1)
			for _, status := range tt.statuses {
				var err error
				if status == nil {
					err = asinfo.ErrNotFound
				}
				calls = append(calls, infoClient.EXPECT().GetBackupStatus(gomock.Any(), fullID).Return(status, err))
			}
			if tt.wantErr != "" {
				calls = append(calls, infoClient.EXPECT().AbortBackup(gomock.Any(), fullID).Return(asinfo.ErrNotFound))
			}
			gomock.InOrder(calls...)

			err := waitForBackup(t.Context(), infoClient, fullID, newBackupOperation(func() {}),
				slog.New(slog.DiscardHandler))

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}
