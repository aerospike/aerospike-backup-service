package serverbackup

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestJobPoller_PollsEveryInterval(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var polls atomic.Int32
		poller := newTestPoller(gomock.NewController(t), func(string) ([]*infomodels.ResponseBackupState, error) {
			polls.Add(1)
			return []*infomodels.ResponseBackupState{}, nil // a cluster with no jobs.
		})
		ctx, cancel := context.WithCancel(t.Context())

		poller.Start(ctx)
		synctest.Wait()
		assert.EqualValues(t, 1, polls.Load(), "polls at once")

		time.Sleep(jobPollInterval)
		synctest.Wait()
		assert.EqualValues(t, 2, polls.Load())

		cancel()
		synctest.Wait() // the poller stops with ctx.
	})
}

func TestJobPoller_LogsJobsAndTheirRoutines(t *testing.T) {
	t.Parallel()

	const after = "1763980000"
	listed := func(id string, state infomodels.BackupState, path, modifiedAfter string) *infomodels.ResponseBackupState {
		listed := job(id, "bar", state)
		listed.Path, listed.ModifiedAfter = path, modifiedAfter

		return listed
	}
	polls := []struct {
		jobs []*infomodels.ResponseBackupState
		err  error
	}{
		{jobs: []*infomodels.ResponseBackupState{
			listed("full", infomodels.BackupStateBaseScanActive, "abs/daily/", ""),
			listed("old", infomodels.BackupStateComplete, "abs/daily/", ""),
			listed("manual", infomodels.BackupStateBaseScanActive, "elsewhere", ""),
		}},
		{err: errors.New("cluster down")},
		{jobs: []*infomodels.ResponseBackupState{
			listed("full", infomodels.BackupStateComplete, "abs/daily/", ""),
			listed("old", infomodels.BackupStateComplete, "abs/daily/", ""),
			listed("manual", infomodels.BackupStateBaseScanActive, "elsewhere", ""),
			listed("incr", infomodels.BackupStateInit, "abs/daily/", after),
		}},
	}

	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		poller := newTestPoller(gomock.NewController(t), func(string) ([]*infomodels.ResponseBackupState, error) {
			poll := polls[calls.Add(1)-1]
			return poll.jobs, poll.err
		})
		ctx, cancel := context.WithCancel(t.Context())

		poller.Start(ctx)
		for range len(polls) - 1 {
			synctest.Wait()
			time.Sleep(jobPollInterval)
		}
		synctest.Wait()
		cancel()
		synctest.Wait()

		// The ended job "old" is not logged on the first poll, a job is logged again only when its
		// state changes, and the failed poll forgets nothing.
		assert.Equal(t, []map[string]string{
			{"jobId": "full", "state": "BASE_SCAN_ACTIVE", "type": "full", "routine": "daily"},
			{"jobId": "manual", "state": "BASE_SCAN_ACTIVE", "type": "full", "routine": ""},
			{"jobId": "full", "state": "COMPLETE", "type": "full", "routine": "daily"},
			{"jobId": "incr", "state": "INIT", "type": "incremental", "routine": "daily"},
		}, poller.logs.jobs())
		assert.Equal(t, 1, poller.logs.count(slog.LevelWarn))
	})
}

// TestJobPoller_FinishesRunsOnce checks that a run that ends is completed and reported once, after
// the listing that shows it ended, with the outcome its jobs tell.
func TestJobPoller_FinishesRunsOnce(t *testing.T) {
	t.Parallel()

	failed := job("job-bar", "bar", infomodels.BackupStateFailed)
	failed.ErrorReason = "disk full"
	aborted := job("job-bar", "bar", infomodels.BackupStateAborted)
	aborted.AbortOrigin = infomodels.AbortOriginOperator
	unread := done("bar")
	unread.RecsReadFailed = 3

	tests := []struct {
		name           string
		final          *infomodels.ResponseBackupState
		wantCompletion string
		wantErr        string
		wantWarnings   int
	}{
		{name: "complete", final: done("bar"), wantCompletion: "success full"},
		{name: "complete without some records", final: unread, wantCompletion: "success full", wantWarnings: 1},
		{
			name: "failed", final: failed, wantCompletion: "failure full",
			wantErr: "full backup failed: namespace bar: server backup job-bar failed: disk full",
		},
		{
			name: "aborted by an operator", final: aborted, wantCompletion: "failure full",
			wantErr: "full backup failed: namespace bar: context canceled: " +
				"server backup job-bar was aborted (abort origin: operator)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				var calls atomic.Int32
				poller := newTestPoller(gomock.NewController(t), func(string) ([]*infomodels.ResponseBackupState, error) {
					if calls.Add(1) == 1 {
						return []*infomodels.ResponseBackupState{at("bar", 50, 1)}, nil
					}

					return []*infomodels.ResponseBackupState{tt.final}, nil
				})
				poller.started()
				ctx, cancel := context.WithCancel(t.Context())

				poller.Start(ctx)
				synctest.Wait()
				assert.Empty(t, poller.outcomes.completions(), "the run is running")

				time.Sleep(3 * jobPollInterval)
				synctest.Wait()
				cancel()
				synctest.Wait()

				assert.Equal(t, []string{tt.wantCompletion}, poller.outcomes.completions())
				reports := poller.outcomes.reports()
				require.Len(t, reports, 1)
				if tt.wantErr == "" {
					require.NoError(t, reports[0])
				} else {
					require.EqualError(t, reports[0], tt.wantErr)
				}
				assert.Equal(t, tt.wantWarnings, poller.logs.count(slog.LevelWarn))
			})
		})
	}
}

// TestJobPoller_CancelAbortsAtOnce checks that canceling a routine lists its cluster at once and
// aborts each job of the routine's runs that runs and is not being aborted.
func TestJobPoller_CancelAbortsAtOnce(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var poller *testPoller
		poller = newTestPoller(gomock.NewController(t), func(string) ([]*infomodels.ResponseBackupState, error) {
			if len(poller.abortedJobs()) > 0 {
				return []*infomodels.ResponseBackupState{job("job-bar", "bar", infomodels.BackupStateAborting)}, nil
			}

			return []*infomodels.ResponseBackupState{at("bar", 50, 1)}, nil
		})
		poller.started()
		ctx, cancel := context.WithCancel(t.Context())
		poller.Start(ctx)
		synctest.Wait()

		time.Sleep(time.Second)
		poller.runs.Cancel(poller.routine.Name)
		synctest.Wait()
		assert.Equal(t, []string{"job-bar"}, poller.abortedJobs(), "aborted a second in, not at the next poll")

		time.Sleep(jobPollInterval)
		synctest.Wait()
		cancel()
		synctest.Wait()

		assert.Equal(t, []string{"job-bar"}, poller.abortedJobs(), "a job being aborted is not aborted again")
	})
}

// TestJobPoller_CanceledRunWaitsForTheServer checks that a canceled run on a cluster that cannot be
// listed goes on showing as running, unreported, until a listing shows how its jobs ended: the
// abort may not have reached them.
func TestJobPoller_CanceledRunWaitsForTheServer(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var reachable atomic.Bool
		poller := newTestPoller(gomock.NewController(t), func(string) ([]*infomodels.ResponseBackupState, error) {
			if !reachable.Load() {
				return nil, errors.New("cluster down")
			}

			aborted := job("job-bar", "bar", infomodels.BackupStateAborted)
			aborted.AbortOrigin = infomodels.AbortOriginOperator

			return []*infomodels.ResponseBackupState{aborted}, nil
		})
		poller.started()
		ctx, cancel := context.WithCancel(t.Context())
		poller.Start(ctx)
		synctest.Wait()

		poller.runs.Cancel(poller.routine.Name)
		time.Sleep(20 * jobPollInterval)
		synctest.Wait()
		var state model.RoutineState
		poller.runs.Describe(poller.routine.Name, &state)
		assert.NotNil(t, state.Full, "the canceled run still runs")
		assert.Empty(t, poller.outcomes.reports())

		reachable.Store(true)
		time.Sleep(jobPollInterval)
		synctest.Wait()
		cancel()
		synctest.Wait()

		reports := poller.outcomes.reports()
		require.Len(t, reports, 1)
		require.ErrorIs(t, reports[0], context.Canceled)
	})
}

// TestJobPoller_PollsClustersOfRuns checks that a cluster no routine backs up any more is polled
// while a run lasts on it, and only then.
func TestJobPoller_PollsClustersOfRuns(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		polls := make(map[string]int)
		poller := newTestPoller(gomock.NewController(t), func(cluster string) ([]*infomodels.ResponseBackupState, error) {
			mu.Lock()
			defer mu.Unlock()
			polls[cluster]++
			if cluster == "other" && polls[cluster] == 1 {
				return []*infomodels.ResponseBackupState{at("bar", 50, 1)}, nil
			}

			return []*infomodels.ResponseBackupState{done("bar")}, nil
		})
		gone := testRoutine("gone")
		gone.SourceCluster = &model.AerospikeCluster{ClusterLabel: "other"}
		require.NoError(t, poller.runs.close(poller.runs.open(gone, model.BackupTypeFull, time.Now()),
			[]startedJob{{namespace: "bar", id: "job-bar"}}, nil))
		ctx, cancel := context.WithCancel(t.Context())

		poller.Start(ctx)
		time.Sleep(2 * jobPollInterval)
		synctest.Wait()
		cancel()
		synctest.Wait()

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, map[string]int{"source": 3, "other": 2}, polls)
		assert.Equal(t, []string{"success full"}, poller.outcomes.completions())
	})
}

// testPoller is a poller of one routine, "daily", on the cluster "source", with what it logged,
// completed, reported and aborted.
type testPoller struct {
	*jobPoller
	routine  *model.BackupRoutine
	logs     *recordedLogs
	outcomes *recordedOutcomes

	mu      sync.Mutex
	aborted []string
}

// newTestPoller returns a testPoller. A listing of a cluster returns what list returns for the
// cluster's label.
func newTestPoller(
	ctrl *gomock.Controller,
	list func(cluster string) ([]*infomodels.ResponseBackupState, error),
) *testPoller {
	poller := &testPoller{routine: testRoutine("daily"), logs: &recordedLogs{}, outcomes: &recordedOutcomes{}}

	// The poller polls one cluster at a time: polled is the one it took a client of last.
	var polled string
	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().ListBackupJobs(gomock.Any()).DoAndReturn(
		func(context.Context) ([]*infomodels.ResponseBackupState, error) { return list(polled) },
	).AnyTimes()
	infoClient.EXPECT().AbortBackup(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, jobID string) error {
			poller.mu.Lock()
			defer poller.mu.Unlock()
			poller.aborted = append(poller.aborted, jobID)

			return nil
		}).AnyTimes()
	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().InfoClient().Return(infoClient).AnyTimes()
	clientManager := aerospike.NewMockClientManager(ctrl)
	clientManager.EXPECT().GetClient(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, cluster *model.AerospikeCluster, _, _ any) (aerospike.Client, error) {
			polled = cluster.ClusterLabel
			return client, nil
		}).AnyTimes()
	clientManager.EXPECT().Close(client).AnyTimes()

	runs := NewRuns()
	poller.jobPoller, _ = NewJobPoller(testRoutines{poller.routine.Name: poller.routine}, clientManager, runs,
		poller.outcomes, poller.outcomes).(*jobPoller)
	poller.logger = slog.New(poller.logs)
	runs.logger = poller.logger

	return poller
}

// started opens and closes a full backup run of the routine whose start began job-bar.
func (p *testPoller) started() {
	opened := p.runs.open(p.routine, model.BackupTypeFull, time.Now())
	if err := p.runs.close(opened, []startedJob{{namespace: "bar", id: "job-bar"}}, nil); err != nil {
		panic(err)
	}
}

func (p *testPoller) abortedJobs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]string(nil), p.aborted...)
}

// recordedOutcomes is the BackupCompletionHandler and the BackupReporter of a testPoller: it keeps
// each completion and the error of each report.
type recordedOutcomes struct {
	mu   sync.Mutex
	done []string
	errs []error
}

func (r *recordedOutcomes) OnSuccess(
	_ context.Context, _ *model.BackupRoutine, backupType model.BackupType, _ time.Time, _ *slog.Logger,
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done = append(r.done, "success "+string(backupType))
}

func (r *recordedOutcomes) OnFailure(_ *model.BackupRoutine, backupType model.BackupType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done = append(r.done, "failure "+string(backupType))
}

func (r *recordedOutcomes) Report(_ string, _ model.BackupType, _ time.Duration, err error, _ *slog.Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, err)
}

func (r *recordedOutcomes) completions() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.done...)
}

func (r *recordedOutcomes) reports() []error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]error(nil), r.errs...)
}

type testRoutines map[string]*model.BackupRoutine

func (r testRoutines) Routines() map[string]*model.BackupRoutine { return r }

// recordedLogs is a slog.Handler that keeps every record.
type recordedLogs struct {
	mu      sync.Mutex
	records []slog.Record
}

func (r *recordedLogs) Enabled(context.Context, slog.Level) bool { return true }

func (r *recordedLogs) Handle(_ context.Context, record slog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.records = append(r.records, record.Clone())

	return nil
}

func (r *recordedLogs) WithAttrs([]slog.Attr) slog.Handler { return r }

func (r *recordedLogs) WithGroup(string) slog.Handler { return r }

// jobs returns the job-id, state, type and routine of each job the poller logged.
func (r *recordedLogs) jobs() []map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var jobs []map[string]string
	for _, record := range r.records {
		if record.Message != "Server backup job" {
			continue
		}

		job := make(map[string]string)
		record.Attrs(func(a slog.Attr) bool {
			switch a.Key {
			case "jobId", "state", "type", "routine":
				job[a.Key] = a.Value.String()
			}
			return true
		})
		jobs = append(jobs, job)
	}

	return jobs
}

// count returns how many records of level were logged.
func (r *recordedLogs) count(level slog.Level) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := 0
	for _, record := range r.records {
		if record.Level == level {
			n++
		}
	}

	return n
}
