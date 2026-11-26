package serverbackup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRoutineRunner_LowerBounds(t *testing.T) {
	t.Parallel()

	last := map[string]time.Time{"bar": time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)}

	tests := []struct {
		name       string
		backupType model.BackupType
		incrMode   model.IncrMode
		wantTypes  []model.BackupType
	}{
		{name: "incremental from the namespace's last backup of either type",
			backupType: model.BackupTypeIncremental,
			wantTypes:  []model.BackupType{model.BackupTypeFull, model.BackupTypeIncremental}},
		{name: "cumulative from the namespace's last full backup", backupType: model.BackupTypeIncremental,
			incrMode: model.IncrModeCumulative, wantTypes: []model.BackupType{model.BackupTypeFull}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			routine := &model.BackupRoutine{
				Name:         "routine",
				BackupPolicy: &model.BackupPolicy{IncrMode: tt.incrMode},
			}
			catalog := NewMockCatalog(gomock.NewController(t))
			catalog.EXPECT().LastBackupTimes(gomock.Any(), routine, tt.wantTypes).Return(last, nil)

			bounds, err := (&routineRunner{catalog: catalog}).lowerBounds(t.Context(), routine, tt.backupType)

			require.NoError(t, err)
			assert.Equal(t, last, bounds)
		})
	}
}

func TestRoutineRunner_LowerBounds_Full(t *testing.T) {
	t.Parallel()

	routine := &model.BackupRoutine{Name: "routine", BackupPolicy: &model.BackupPolicy{}}

	// The mock fails the test on any call: a full backup reads no storage.
	bounds, err := (&routineRunner{catalog: NewMockCatalog(gomock.NewController(t))}).
		lowerBounds(t.Context(), routine, model.BackupTypeFull)

	require.NoError(t, err)
	assert.Nil(t, bounds)
	assert.Nil(t, lowerBound(bounds, "bar"))
}

func TestRoutineRunner_LowerBounds_ReadFails(t *testing.T) {
	t.Parallel()

	catalog := NewMockCatalog(gomock.NewController(t))
	catalog.EXPECT().LastBackupTimes(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("storage down"))
	routine := &model.BackupRoutine{Name: "routine", BackupPolicy: &model.BackupPolicy{}}

	got, err := (&routineRunner{catalog: catalog}).lowerBounds(t.Context(), routine, model.BackupTypeIncremental)

	require.EqualError(t, err, "failed to read the last backups of routine routine: storage down")
	assert.Nil(t, got)
}

// expectStartedJobs records each backup request and names its job job-<namespace>.
func expectStartedJobs(infoClient *aerospike.MockInfoGetter, requests *[]infomodels.RequestBackup) {
	var mu sync.Mutex // the namespaces' jobs start at once.
	infoClient.EXPECT().StartBackup(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *infomodels.RequestBackup) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			*requests = append(*requests, *request)

			return "job-" + request.Namespace, nil
		}).AnyTimes()
}

// TestRoutineRunner_Run_NamespacesTogether checks that a run starts the jobs of its namespaces at
// once, weighs its progress by the number of namespaces, and counts each one as it ends.
func TestRoutineRunner_Run_NamespacesTogether(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoClient := aerospike.NewMockInfoGetter(ctrl)
		infoClient.EXPECT().StartBackup(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, request *infomodels.RequestBackup) (string, error) {
				return "job-" + request.Namespace, nil
			}).Times(2)
		// job-baz completes at the first poll; job-bar is half way at the first and completes at
		// the second.
		jobs := newFakePoller(func(poll int) map[string]*infomodels.ResponseBackupState {
			bar := complete("job-bar")
			if poll == 1 {
				bar = &infomodels.ResponseBackupState{
					JobID: "job-bar", State: infomodels.BackupStateBaseScanActive, ProgressPct: 50,
				}
			}

			return listed(bar, complete("job-baz"))
		})
		runner, routine := newTestRunner(ctrl, infoClient, jobs, "bar", "baz")

		handler, err := runner.Run(t.Context(), routine, model.BackupRunSpec{Type: model.BackupTypeFull}, discardLogger)
		require.NoError(t, err)

		time.Sleep(jobPollInterval)
		synctest.Wait()
		assert.InDelta(t, 0.75, handler.Status().Progress, 1e-9, "baz is done and bar half way")

		require.NoError(t, handler.Wait(t.Context()))
		assert.InDelta(t, 1, handler.Status().Progress, 1e-9, "both namespaces are done")

		synctest.Wait() // the run releases its client after it reports its outcome.
	})
}

// TestRoutineRunner_Run_OutlivesContext checks that a run goes on when the context it started on
// ends, as it does when the service stops: no job is aborted and every namespace is backed up.
func TestRoutineRunner_Run_OutlivesContext(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		var requests []infomodels.RequestBackup
		infoClient := aerospike.NewMockInfoGetter(ctrl)
		expectStartedJobs(infoClient, &requests)
		var aborts atomic.Int32
		infoClient.EXPECT().AbortBackup(gomock.Any(), gomock.Any()).DoAndReturn(
			func(context.Context, string) error {
				aborts.Add(1)

				return nil
			}).AnyTimes()
		runner, routine := newTestRunner(ctrl, infoClient, newFakePoller(completeJobs("bar", "baz")), "bar", "baz")
		ctx, cancel := context.WithCancel(t.Context())

		handler, err := runner.Run(ctx, routine, model.BackupRunSpec{Type: model.BackupTypeFull}, discardLogger)
		require.NoError(t, err)
		cancel()

		require.NoError(t, handler.Wait(t.Context()))
		assert.Zero(t, aborts.Load(), "aborted jobs")
		assert.Len(t, requests, 2)

		synctest.Wait() // the run releases its client after it reports its outcome.
	})
}

// TestRoutineRunner_Run_CancelAborts checks that canceling a run through its handler aborts every
// job of the run that is running.
func TestRoutineRunner_Run_CancelAborts(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		infoClient := aerospike.NewMockInfoGetter(ctrl)
		var mu sync.Mutex
		aborted := make(map[string]bool)
		infoClient.EXPECT().StartBackup(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, request *infomodels.RequestBackup) (string, error) {
				return "job-" + request.Namespace, nil
			}).Times(2)
		jobs := newFakePoller(func(int) map[string]*infomodels.ResponseBackupState {
			mu.Lock()
			defer mu.Unlock()
			listing := make(map[string]*infomodels.ResponseBackupState)
			for _, jobID := range []string{"job-bar", "job-baz"} {
				state := infomodels.BackupStateBaseScanActive
				if aborted[jobID] {
					state = infomodels.BackupStateAborted
				}
				listing[jobID] = &infomodels.ResponseBackupState{JobID: jobID, State: state}
			}

			return listing
		})
		infoClient.EXPECT().AbortBackup(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, jobID string) error {
				mu.Lock()
				defer mu.Unlock()
				aborted[jobID] = true

				return nil
			}).Times(2)
		runner, routine := newTestRunner(ctrl, infoClient, jobs, "bar", "baz")

		handler, err := runner.Run(t.Context(), routine, model.BackupRunSpec{Type: model.BackupTypeFull}, discardLogger)
		require.NoError(t, err)
		time.Sleep(jobPollInterval)
		synctest.Wait()
		require.Len(t, handler.Status().Server.Jobs, 2, "both jobs have reported")
		handler.Cancel()

		require.ErrorIs(t, handler.Wait(t.Context()), context.Canceled)
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, map[string]bool{"job-bar": true, "job-baz": true}, aborted)

		synctest.Wait() // the run releases its client after it reports its outcome.
	})
}

// newTestRunner returns a runner that backs namespaces up through infoClient and follows its jobs
// through jobs, and its routine. It expects one run: the client is taken once and released once.
func newTestRunner(
	ctrl *gomock.Controller, infoClient *aerospike.MockInfoGetter, jobs JobPoller, namespaces ...string,
) (*routineRunner, *model.BackupRoutine) {
	routine := &model.BackupRoutine{
		Name:          "routine",
		SourceCluster: &model.AerospikeCluster{},
		Storage:       &model.S3Storage{Bucket: "bucket", S3Region: "us-east-1"},
		BackupPolicy:  &model.BackupPolicy{},
	}
	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().InfoClient().Return(infoClient)
	clientManager := aerospike.NewMockClientManager(ctrl)
	clientManager.EXPECT().GetClient(gomock.Any(), routine.SourceCluster, gomock.Any(), gomock.Any()).
		Return(client, nil)
	clientManager.EXPECT().Close(client)
	namespaceResolver := aerospike.NewMockNamespaceResolver(ctrl)
	namespaceResolver.EXPECT().ResolveNamespaces(gomock.Any(), routine, gomock.Any()).Return(namespaces, nil)

	return &routineRunner{clientManager: clientManager, namespaceResolver: namespaceResolver, jobs: jobs}, routine
}

func TestRoutineRunner_BackupNamespaces_Requests(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	at := func(minutes int) time.Time { return start.Add(time.Duration(minutes) * time.Minute) }
	unix := func(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }

	storage := &model.S3Storage{Bucket: "bucket", S3Region: "eu-central-1", Path: "backups", S3Profile: "backup"}
	common := func(namespace string) infomodels.RequestCommon {
		return infomodels.RequestCommon{
			Namespace: namespace, Storage: "aws-s3", Path: "backups/routine", Bucket: "bucket",
			Region: "eu-central-1", Profile: "backup",
		}
	}

	lastBackups := map[string]time.Time{"bar": at(10), "baz": at(1)}

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
				Name:             "routine",
				Storage:          storage,
				SetList:          []string{"setA", "setB"},
				BinList:          []string{"bin1", "bin2"},
				FilterExpression: "kxkCkwECAA==",
				BackupPolicy: &model.BackupPolicy{
					RecordsPerSecond:  new(100),
					Parallel:          new(2),
					ParallelWrite:     new(3),
					NoIndexes:         new(true),
					NoUdfs:            new(false),
					WithClusterConfig: new(true),
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
				NoConfig:            new(false),
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
				{RequestCommon: common("bar"), NoConfig: new(true), ModifiedAfter: unix(at(10))},
				{RequestCommon: common("baz"), NoConfig: new(true), ModifiedAfter: unix(at(1))},
			},
		},
		{
			name:       "full without the cluster configuration",
			routine:    &model.BackupRoutine{Name: "routine", Storage: storage, BackupPolicy: &model.BackupPolicy{}},
			runSpec:    model.BackupRunSpec{Type: model.BackupTypeFull},
			namespaces: []string{"bar"},
			want:       []infomodels.RequestBackup{{RequestCommon: common("bar"), NoConfig: new(true)}},
		},
		{
			// Only a full backup stores the cluster configuration.
			name: "incremental with the cluster configuration",
			routine: &model.BackupRoutine{
				Name: "routine", Storage: storage,
				BackupPolicy: &model.BackupPolicy{WithClusterConfig: new(true)},
			},
			runSpec:    model.BackupRunSpec{Type: model.BackupTypeIncremental},
			namespaces: []string{"bar"},
			want: []infomodels.RequestBackup{
				{RequestCommon: common("bar"), NoConfig: new(true), ModifiedAfter: unix(at(10))},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctrl := gomock.NewController(t)

				catalog := NewMockCatalog(ctrl)
				catalog.EXPECT().LastBackupTimes(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(lastBackups, nil).MaxTimes(1) // once per run, not once per namespace.

				var requests []infomodels.RequestBackup
				infoClient := aerospike.NewMockInfoGetter(ctrl)
				expectStartedJobs(infoClient, &requests)
				operation := newBackupOperation(func() {}, len(tt.namespaces))
				runner := &routineRunner{catalog: catalog, jobs: newFakePoller(completeJobs(tt.namespaces...))}

				err := runner.backupNamespaces(t.Context(), infoClient,
					tt.routine, tt.runSpec, tt.namespaces, operation, discardLogger)

				require.NoError(t, err)
				assert.ElementsMatch(t, tt.want, requests)
				jobIDs := make([]string, 0, len(tt.namespaces))
				for _, job := range operation.Status().Server.Jobs {
					jobIDs = append(jobIDs, job.JobID)
				}
				assert.Len(t, jobIDs, len(tt.namespaces), "one server job per namespace")
				for i, namespace := range tt.namespaces {
					assert.Equal(t, "job-"+namespace, jobIDs[i], "the jobs are listed in the namespaces' order")
				}
				assert.InDelta(t, 1, operation.Status().Progress, 1e-9, "every namespace counts as done")
			})
		})
	}
}

// TestRoutineRunner_BackupNamespaces_Failures checks that a namespace that fails does not stop the
// others, and that the run fails with the error of each namespace that failed.
func TestRoutineRunner_BackupNamespaces_Failures(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		infoClient := aerospike.NewMockInfoGetter(ctrl)
		infoClient.EXPECT().StartBackup(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, request *infomodels.RequestBackup) (string, error) {
				if request.Namespace == "bar" {
					return "", errors.New("refused")
				}

				return "job-" + request.Namespace, nil
			}).Times(3)
		jobs := newFakePoller(func(int) map[string]*infomodels.ResponseBackupState {
			return listed(
				&infomodels.ResponseBackupState{
					JobID: "job-baz", Namespace: "baz", State: infomodels.BackupStateFailed, ErrorReason: "disk full",
				},
				&infomodels.ResponseBackupState{
					JobID: "job-qux", Namespace: "qux", State: infomodels.BackupStateComplete, ProgressPct: 100,
				},
			)
		})
		routine := &model.BackupRoutine{
			Storage:      &model.S3Storage{Bucket: "bucket", S3Region: "us-east-1"},
			BackupPolicy: &model.BackupPolicy{},
		}
		operation := newBackupOperation(func() {}, 3)

		err := (&routineRunner{jobs: jobs}).backupNamespaces(t.Context(), infoClient, routine,
			model.BackupRunSpec{Type: model.BackupTypeFull}, []string{"bar", "baz", "qux"}, operation,
			discardLogger)

		require.EqualError(t, err, "namespace bar: failed to start server backup: refused\n"+
			"namespace baz: server backup job-baz failed: disk full")
		assert.Equal(t, []model.ServerBackupJob{
			{JobID: "job-baz", Namespace: "baz", State: "FAILED", ErrorReason: "disk full"},
			{JobID: "job-qux", Namespace: "qux", State: "COMPLETE"},
		}, operation.Status().Server.Jobs)
		assert.InDelta(t, 1, operation.Status().Progress, 1e-9, "a failed namespace counts as done")
	})
}

// TestRoutineRunner_BackupNamespaces_StartOutcomeUnknown checks that a start whose outcome is
// unknown aborts the job it may have started, and that a start the server refused aborts nothing.
func TestRoutineRunner_BackupNamespaces_StartOutcomeUnknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		startErr  error
		wantAbort bool
	}{
		{name: "outcome unknown", startErr: fmt.Errorf("%w: timeout", asinfo.ErrOutcomeUnknown), wantAbort: true},
		{name: "refused", startErr: errors.New("refused")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
			infoClient.EXPECT().StartBackup(gomock.Any(), gomock.Any()).Return(fullID, tt.startErr)
			if tt.wantAbort {
				// A refused abort is not followed: there is nothing to wait for.
				infoClient.EXPECT().AbortBackup(gomock.Any(), fullID).Return(errors.New("not principal"))
			}
			routine := &model.BackupRoutine{Storage: &model.S3Storage{Bucket: "bucket"}, BackupPolicy: &model.BackupPolicy{}}

			err := (&routineRunner{}).backupNamespaces(t.Context(), infoClient, routine,
				model.BackupRunSpec{Type: model.BackupTypeFull}, []string{"bar"}, newBackupOperation(func() {}, 1),
				discardLogger)

			require.ErrorIs(t, err, tt.startErr)
		})
	}
}

func TestRoutineRunner_BackupNamespaces_NotS3(t *testing.T) {
	t.Parallel()

	// Nothing is sent to the server.
	infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
	routine := &model.BackupRoutine{Storage: &model.LocalStorage{Path: "/backups"}, BackupPolicy: &model.BackupPolicy{}}

	err := (&routineRunner{}).backupNamespaces(t.Context(), infoClient, routine,
		model.BackupRunSpec{Type: model.BackupTypeFull}, []string{"bar"}, newBackupOperation(func() {}, 1),
		discardLogger)

	require.EqualError(t, err, "namespace bar: server backup requires S3 storage, got *model.LocalStorage")
}

func TestBackupJob_Wait_RecordsServerJob(t *testing.T) {
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
			wantErr: "server backup " + fullID + " was aborted (abort origin: cluster): node left",
			wantJob: model.ServerBackupJob{
				JobID: fullID, Namespace: "test", State: "ABORTED", ErrorReason: "node left", AbortOrigin: "cluster",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				jobs := newFakePoller(func(poll int) map[string]*infomodels.ResponseBackupState {
					if poll == 1 {
						return listed(running)
					}

					return listed(tt.final)
				})
				operation := newBackupOperation(func() {}, 1)

				err := newTestBackupJob(nil, operation).Wait(t.Context(), jobs)

				if tt.wantErr == "" {
					require.NoError(t, err)
				} else {
					require.ErrorContains(t, err, tt.wantErr)
				}
				assert.Equal(t, []model.ServerBackupJob{tt.wantJob}, operation.Status().Server.Jobs)
			})
		})
	}
}

func TestBackupJob_Wait_WarnsOfUnreadRecords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		recsReadFailed uint64
		wantLog        string
	}{
		{name: "all records read"},
		{
			name:           "records not read",
			recsReadFailed: 3,
			wantLog:        "level=WARN.*jobId=" + fullID + " namespace=test recordsReadFailed=3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				jobs := newFakePoller(func(int) map[string]*infomodels.ResponseBackupState {
					return listed(&infomodels.ResponseBackupState{
						JobID: fullID, Namespace: "test", State: infomodels.BackupStateComplete,
						RecsReadFailed: tt.recsReadFailed,
					})
				})
				var logs bytes.Buffer
				job := newTestBackupJob(nil, newBackupOperation(func() {}, 1))
				job.logger = slog.New(slog.NewTextHandler(&logs, nil))

				require.NoError(t, job.Wait(t.Context(), jobs))

				if tt.wantLog == "" {
					assert.Empty(t, logs.String())
					return
				}
				assert.Regexp(t, tt.wantLog, logs.String())
			})
		})
	}
}

func TestBackupJob_Wait_Canceled_AbortRefused(t *testing.T) {
	t.Parallel()

	// The job's status is not looked up: the abort did not reach it.
	infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
	infoClient.EXPECT().AbortBackup(gomock.Any(), fullID).Return(errors.New("not principal"))
	operation := newBackupOperation(func() {}, 1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := newTestBackupJob(infoClient, operation).Wait(ctx, newFakePoller(completeJobs()))

	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, operation.Status().Server.Jobs)
}

func TestBackupJob_Wait_Canceled_RecordsAbortedJob(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
		infoClient.EXPECT().AbortBackup(gomock.Any(), fullID).Return(nil)
		// The server reports the abort origin once the abort has ended the job.
		jobs := newFakePoller(func(poll int) map[string]*infomodels.ResponseBackupState {
			if poll == 1 {
				return listed(&infomodels.ResponseBackupState{
					JobID: fullID, Namespace: "test", State: infomodels.BackupStateAborting,
				})
			}

			return listed(&infomodels.ResponseBackupState{
				JobID: fullID, Namespace: "test", State: infomodels.BackupStateAborted,
				AbortOrigin: infomodels.AbortOriginOperator,
			})
		})
		operation := newBackupOperation(func() {}, 1)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := newTestBackupJob(infoClient, operation).Wait(ctx, jobs)

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, []model.ServerBackupJob{
			{JobID: fullID, Namespace: "test", State: "ABORTED", AbortOrigin: "operator"},
		}, operation.Status().Server.Jobs)
	})
}

// TestBackupOperation_ServerJobs_OnePerJob checks that the operation lists each namespace's job
// once, as last reported, in the namespaces' order whatever order they report in.
func TestBackupOperation_ServerJobs_OnePerJob(t *testing.T) {
	t.Parallel()
	operation := newBackupOperation(func() {}, 2)

	status := func(jobID, namespace string, state infomodels.BackupState) *infomodels.ResponseBackupState {
		return &infomodels.ResponseBackupState{JobID: jobID, Namespace: namespace, State: state}
	}

	operation.record(1, status(incr1ID, "baz", infomodels.BackupStateFailed))
	operation.record(0, status(fullID, "bar", infomodels.BackupStateInit))
	operation.record(0, status(fullID, "bar", infomodels.BackupStateComplete))

	assert.Equal(t, []model.ServerBackupJob{
		{JobID: fullID, Namespace: "bar", State: "COMPLETE"},
		{JobID: incr1ID, Namespace: "baz", State: "FAILED"},
	}, operation.Status().Server.Jobs)
}

func TestBackupOperation_ReportsProgress(t *testing.T) {
	t.Parallel()
	operation := newBackupOperation(func() {}, 1)

	// The server reports percent, the operation a fraction, and no scan slot: it counts no records.
	for _, percent := range []float64{0, 42.7, 95, 100} {
		operation.record(0, &infomodels.ResponseBackupState{JobID: fullID, ProgressPct: percent})

		run := operation.Status()
		assert.InDelta(t, percent/100, run.Progress, 1e-9, "progress %v", percent)
		assert.Nil(t, run.Scan)
	}
}

// TestBackupOperation_ProgressAcrossNamespaces checks that every namespace of a run weighs the
// same, that one whose job has not reported counts as not started, and that a finished one counts
// whole whatever its job last reported.
func TestBackupOperation_ProgressAcrossNamespaces(t *testing.T) {
	t.Parallel()
	operation := newBackupOperation(func() {}, 2)

	operation.record(1, &infomodels.ResponseBackupState{JobID: incr1ID, ProgressPct: 40})
	assert.InDelta(t, 0.2, operation.Status().Progress, 1e-9, "40% of the second namespace")

	operation.record(0, &infomodels.ResponseBackupState{JobID: fullID, ProgressPct: 50})
	assert.InDelta(t, 0.45, operation.Status().Progress, 1e-9, "half of the first namespace, 40% of the second")

	operation.finishNamespace(1)
	assert.InDelta(t, 0.75, operation.Status().Progress, 1e-9, "the second namespace done")
}

// TestBackupOperation_ReportsThroughput checks that the operation reports the sum of the running
// jobs' record and upload rates in a snapshot of its own, and drops a job's rates once its
// namespace is done.
func TestBackupOperation_ReportsThroughput(t *testing.T) {
	t.Parallel()
	operation := newBackupOperation(func() {}, 2)

	before := operation.Status().Server.Metrics
	assert.Equal(t, &models.Metrics{}, before, "no job has reported")

	operation.record(0, &infomodels.ResponseBackupState{JobID: fullID, RecsPerSec: 1200, BytesOutPerSec: 2048 * 1024})
	operation.record(1, &infomodels.ResponseBackupState{JobID: incr1ID, RecsPerSec: 300, BytesOutPerSec: 1024 * 1024})
	assert.Equal(t, &models.Metrics{RecordsPerSecond: 1500, KilobytesPerSecond: 3072},
		operation.Status().Server.Metrics)
	assert.Equal(t, &models.Metrics{}, before, "a snapshot does not change")

	operation.finishNamespace(0)
	assert.Equal(t, &models.Metrics{RecordsPerSecond: 300, KilobytesPerSecond: 1024},
		operation.Status().Server.Metrics, "the first namespace is done")
}

// TestBackupJob_Wait_MissingFromPolls checks that a job one poll leaves out is followed on, that
// two polls in a row leaving it out lose it without an abort, and that a listing resets the count.
func TestBackupJob_Wait_MissingFromPolls(t *testing.T) {
	t.Parallel()

	running := &infomodels.ResponseBackupState{JobID: fullID, State: infomodels.BackupStateBaseScanActive}
	tests := []struct {
		name string
		// polls lists the job at each poll, nil for a poll that leaves it out.
		polls   []*infomodels.ResponseBackupState
		wantErr string
	}{
		{
			name:  "missed once, then complete",
			polls: []*infomodels.ResponseBackupState{nil, running, nil, complete(fullID)},
		},
		{
			name:    "missed twice in a row",
			polls:   []*infomodels.ResponseBackupState{running, nil, nil},
			wantErr: "server backup " + fullID + " is lost: the server does not list it",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				// A lost job is not aborted: no abort is expected.
				infoClient := aerospike.NewMockInfoGetter(gomock.NewController(t))
				jobs := newFakePoller(func(poll int) map[string]*infomodels.ResponseBackupState {
					if job := tt.polls[poll-1]; job != nil {
						return listed(job)
					}

					return listed()
				})

				err := newTestBackupJob(infoClient, newBackupOperation(func() {}, 1)).Wait(t.Context(), jobs)

				if tt.wantErr == "" {
					require.NoError(t, err)
					return
				}
				require.EqualError(t, err, tt.wantErr)
			})
		})
	}
}

// newTestBackupJob returns the backup job fullID, which records its statuses in operation as the
// job of its first namespace.
func newTestBackupJob(infoClient *aerospike.MockInfoGetter, operation *backupOperation) *backupJob {
	record := func(status *infomodels.ResponseBackupState) { operation.record(0, status) }

	return &backupJob{infoClient: infoClient, jobID: fullID, record: record, logger: discardLogger}
}

// fakePoller polls every jobPollInterval from when it is made: poll n begins and lands n
// intervals later, and lists jobs(n).
type fakePoller struct {
	start time.Time
	jobs  func(poll int) map[string]*infomodels.ResponseBackupState
}

func newFakePoller(jobs func(poll int) map[string]*infomodels.ResponseBackupState) *fakePoller {
	return &fakePoller{start: time.Now(), jobs: jobs}
}

func (f *fakePoller) Start(context.Context) {}

func (f *fakePoller) Next(ctx context.Context, _ *model.AerospikeCluster, after time.Time) (JobListing, error) {
	poll := 1
	if after.After(f.start) {
		poll = int(after.Sub(f.start)/jobPollInterval) + 1
	}
	at := f.start.Add(time.Duration(poll) * jobPollInterval)

	select {
	case <-ctx.Done():
		return JobListing{}, ctx.Err()
	case <-time.After(time.Until(at)):
	}

	return JobListing{Jobs: f.jobs(poll), At: at}, nil
}

// completeJobs lists job-<namespace> of each namespace complete at every poll.
func completeJobs(namespaces ...string) func(int) map[string]*infomodels.ResponseBackupState {
	return func(int) map[string]*infomodels.ResponseBackupState {
		listing := make(map[string]*infomodels.ResponseBackupState, len(namespaces))
		for _, namespace := range namespaces {
			listing["job-"+namespace] = complete("job-" + namespace)
		}

		return listing
	}
}

// complete returns the status of jobID complete at 100%.
func complete(jobID string) *infomodels.ResponseBackupState {
	return &infomodels.ResponseBackupState{JobID: jobID, State: infomodels.BackupStateComplete, ProgressPct: 100}
}

// listed returns a poll's listing of jobs.
func listed(jobs ...*infomodels.ResponseBackupState) map[string]*infomodels.ResponseBackupState {
	return jobsByID(jobs)
}
