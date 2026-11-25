package serverbackup

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/backup-go/models"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// testRuns is a Runs of the routine "daily" with the listings of its cluster, each beginning
// jobPollInterval after the one before.
type testRuns struct {
	*Runs
	routine *model.BackupRoutine
	at      time.Time
}

// newTestRuns returns a testRuns whose first listing begins after now.
func newTestRuns() *testRuns {
	return &testRuns{Runs: NewRuns(), routine: testRoutine("daily"), at: time.Now()}
}

// list hands the runs the next listing of the cluster, of jobs.
func (r *testRuns) list(jobs ...*infomodels.ResponseBackupState) (ended []*run, aborts []string) {
	r.at = r.at.Add(jobPollInterval)

	return r.update(r.routine.SourceCluster, jobs, r.at, testRoutines{r.routine.Name: r.routine})
}

// started opens and closes a run of the routine of backupType whose start began jobs, one per
// namespace, named job-<namespace>.
func (r *testRuns) started(backupType model.BackupType, namespaces ...string) {
	jobs := make([]startedJob, 0, len(namespaces))
	for _, namespace := range namespaces {
		jobs = append(jobs, startedJob{namespace: namespace, id: "job-" + namespace})
	}

	if err := r.close(r.open(r.routine, backupType, time.Now()), jobs, nil); err != nil {
		panic(err)
	}
}

func (r *testRuns) state() model.RoutineState {
	var state model.RoutineState
	r.Describe(r.routine.Name, &state)

	return state
}

// testRoutine returns the routine name on the cluster "source", which backs up to the bucket
// "bucket" under "abs": the server reports its path as abs/<name>/.
func testRoutine(name string) *model.BackupRoutine {
	return &model.BackupRoutine{
		Name:          name,
		SourceCluster: &model.AerospikeCluster{ClusterLabel: "source"},
		Storage:       &model.S3Storage{Bucket: "bucket", Path: "abs"},
		BackupPolicy:  &model.BackupPolicy{},
	}
}

// job returns the listing of the full backup job id of namespace in state, in the path of the
// routine "daily".
func job(id, namespace string, state infomodels.BackupState) *infomodels.ResponseBackupState {
	return &infomodels.ResponseBackupState{
		JobID: id, Namespace: namespace, State: state, S3Bucket: "bucket", Path: "abs/daily/",
	}
}

// at returns the listing of the job of namespace, running at percent with throughput.
func at(namespace string, percent float64, recordsPerSecond uint64) *infomodels.ResponseBackupState {
	listed := job("job-"+namespace, namespace, infomodels.BackupStateBaseScanActive)
	listed.ProgressPct, listed.RecsPerSec, listed.BytesOutPerSec = percent, recordsPerSecond, 1024*recordsPerSecond

	return listed
}

// done returns the listing of the job of namespace, complete.
func done(namespace string) *infomodels.ResponseBackupState {
	listed := job("job-"+namespace, namespace, infomodels.BackupStateComplete)
	listed.ProgressPct = 100

	return listed
}

func jobIDs(jobs []model.ServerBackupJob) []string {
	ids := make([]string, 0, len(jobs))
	for _, job := range jobs {
		ids = append(ids, job.JobID)
	}

	return ids
}

// TestRuns_StartedRunEndsWithItsJobs follows a run of two namespaces from its start to its end:
// it shows as running before it is listed, weighs its namespaces the same, lists its jobs in their
// order, never falls back when a namespace ends, and ends once, at the listing that shows both
// done, leaving its jobs as the last ones of its type.
func TestRuns_StartedRunEndsWithItsJobs(t *testing.T) {
	t.Parallel()

	runs := newTestRuns()
	runs.started(model.BackupTypeFull, "bar", "baz")

	running := runs.state().Full
	require.NotNil(t, running, "a started backup runs before it is listed")
	assert.Zero(t, running.Progress)
	assert.Empty(t, running.ServerJobs)

	ended, _ := runs.list(at("baz", 80, 30), at("bar", 60, 10))
	assert.Empty(t, ended)
	running = runs.state().Full
	assert.InDelta(t, 0.7, running.Progress, 1e-9)
	assert.Equal(t, []string{"job-bar", "job-baz"}, jobIDs(running.ServerJobs), "in the namespaces' order")
	assert.Equal(t, &models.Metrics{RecordsPerSecond: 40, KilobytesPerSecond: 40}, running.Metrics)

	ended, _ = runs.list(at("bar", 60, 10), done("baz"))
	assert.Empty(t, ended)
	running = runs.state().Full
	assert.InDelta(t, 0.8, running.Progress, 1e-9, "a namespace that ended counts whole")
	assert.Equal(t, &models.Metrics{RecordsPerSecond: 10, KilobytesPerSecond: 10}, running.Metrics)

	ended, _ = runs.list(done("bar"), done("baz"))
	require.Len(t, ended, 1)
	require.NoError(t, ended[0].outcome())
	state := runs.state()
	assert.Nil(t, state.Full)
	assert.Equal(t, []string{"job-bar", "job-baz"}, jobIDs(state.LastFullJobs))
	assert.Empty(t, state.LastIncrementalJobs)

	ended, _ = runs.list(done("bar"), done("baz"))
	assert.Empty(t, ended, "a run ends once")
}

// TestRuns_StartThatBeganNoJob checks that a run whose start began no job ends at the start, with
// the start's error, and leaves nothing behind.
func TestRuns_StartThatBeganNoJob(t *testing.T) {
	t.Parallel()

	runs := newTestRuns()
	startErr := errors.New("namespace bar: refused")

	err := runs.close(runs.open(runs.routine, model.BackupTypeFull, time.Now()), nil, startErr)

	require.ErrorIs(t, err, startErr)
	assert.Nil(t, runs.state().Full)
	ended, _ := runs.list()
	assert.Empty(t, ended)
}

// TestRuns_Outcome checks the error a run ends with: one per namespace whose job did not start or
// did not complete. A job an operator aborted makes the run canceled.
func TestRuns_Outcome(t *testing.T) {
	t.Parallel()

	aborted := func(origin infomodels.AbortOrigin) *infomodels.ResponseBackupState {
		listed := job("job-bar", "bar", infomodels.BackupStateAborted)
		listed.AbortOrigin, listed.ErrorReason = origin, "stopped"

		return listed
	}
	failed := job("job-bar", "bar", infomodels.BackupStateFailed)
	failed.ErrorReason = "disk full"

	tests := []struct {
		name         string
		startErr     error
		listed       *infomodels.ResponseBackupState
		wantErr      string
		wantCanceled bool
	}{
		{name: "complete", listed: done("bar")},
		{
			name:    "failed",
			listed:  failed,
			wantErr: "namespace bar: server backup job-bar failed: disk full",
		},
		{
			name:    "aborted by the server",
			listed:  aborted(infomodels.AbortOriginServer),
			wantErr: "namespace bar: server backup job-bar was aborted (abort origin: server): stopped",
		},
		{
			name:         "aborted by an operator",
			listed:       aborted(infomodels.AbortOriginOperator),
			wantErr:      "namespace bar: context canceled: server backup job-bar was aborted (abort origin: operator): stopped",
			wantCanceled: true,
		},
		{
			name:     "a namespace that did not start",
			startErr: errors.New("namespace baz: failed to start server backup: refused"),
			listed:   done("bar"),
			wantErr:  "namespace baz: failed to start server backup: refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runs := newTestRuns()
			opened := runs.open(runs.routine, model.BackupTypeFull, time.Now())
			require.NoError(t, runs.close(opened, []startedJob{{namespace: "bar", id: "job-bar"}}, tt.startErr))

			ended, _ := runs.list(tt.listed)

			require.Len(t, ended, 1)
			err := ended[0].outcome()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
			assert.Equal(t, tt.wantCanceled, errors.Is(err, context.Canceled))
		})
	}
}

// TestRuns_LostJob checks that a job two listings in a row leave out is lost, that only a listing
// that began after the run learned of the job counts, and that a listing that shows it resets the
// count.
func TestRuns_LostJob(t *testing.T) {
	t.Parallel()

	runs := newTestRuns()
	runs.started(model.BackupTypeFull, "bar")
	runs.at = time.Now().Add(-3 * jobPollInterval / 2) // the first listing began before the start.

	for _, listing := range [][]*infomodels.ResponseBackupState{
		{},                 // began before the start answered.
		{},                 // missed once.
		{at("bar", 10, 1)}, // listed.
		{},                 // missed once again.
	} {
		ended, _ := runs.list(listing...)
		require.Empty(t, ended)
	}

	ended, _ := runs.list()
	require.Len(t, ended, 1, "missed twice in a row")
	require.EqualError(t, ended[0].outcome(), "namespace bar: server backup job-bar is lost: the server does not list it")
	assert.Equal(t, []string{"job-bar"}, jobIDs(runs.state().LastFullJobs), "as last listed")
}

// TestRuns_StartingRunDoesNotEnd checks that a run does not end, nor is its job taken for one this
// process did not start, while its start is starting its jobs.
func TestRuns_StartingRunDoesNotEnd(t *testing.T) {
	t.Parallel()

	runs := newTestRuns()
	opened := runs.open(runs.routine, model.BackupTypeFull, time.Now())

	ended, _ := runs.list(done("bar"))
	assert.Empty(t, ended)
	require.NoError(t, runs.close(opened, []startedJob{{namespace: "bar", id: "job-bar"}}, nil))

	ended, _ = runs.list(done("bar"))
	require.Len(t, ended, 1)
	assert.Same(t, opened, ended[0])
}

// TestRuns_StartingRoutineTakesOnNoJob checks that a listing taken while a start of the routine
// is starting takes on none of the routine's jobs, whatever type they look like: an incremental
// backup of a namespace with no backup to build on has no lower bound, as a full one.
func TestRuns_StartingRoutineTakesOnNoJob(t *testing.T) {
	t.Parallel()

	runs := newTestRuns()
	opened := runs.open(runs.routine, model.BackupTypeIncremental, time.Now())

	ended, _ := runs.list(at("new", 10, 1))
	require.Empty(t, ended)
	require.NoError(t, runs.close(opened, []startedJob{{namespace: "new", id: "job-new"}}, nil))

	state := runs.state()
	assert.Nil(t, state.Full)
	require.NotNil(t, state.Incremental)

	ended, _ = runs.list(done("new"))
	require.Len(t, ended, 1, "one backup, one report")
	assert.Equal(t, model.BackupTypeIncremental, ended[0].backupType)
}

// TestRuns_FollowsTheRoutineAsConfigured checks that a run follows its routine through the
// configuration of each listing, so an edited cluster entry is listed, and that a run whose
// routine is gone keeps the cluster it had.
func TestRuns_FollowsTheRoutineAsConfigured(t *testing.T) {
	t.Parallel()

	runs := newTestRuns()
	runs.started(model.BackupTypeFull, "bar")
	edited := testRoutine("daily")
	edited.SourceCluster = &model.AerospikeCluster{ClusterLabel: "source", ConnTimeout: new(time.Second)}
	require.NotEqual(t, runs.routine.SourceCluster.Hash(), edited.SourceCluster.Hash())
	routines := testRoutines{"daily": edited}

	assert.Equal(t, []*model.AerospikeCluster{edited.SourceCluster}, runs.clusters(routines))
	ended, _ := runs.update(edited.SourceCluster, []*infomodels.ResponseBackupState{done("bar")},
		time.Now().Add(time.Second), routines)
	require.Len(t, ended, 1)
	assert.Same(t, edited, ended[0].routine)

	runs = newTestRuns()
	runs.started(model.BackupTypeFull, "bar")
	assert.Equal(t, []*model.AerospikeCluster{runs.routine.SourceCluster}, runs.clusters(testRoutines{}))
	ended, _ = runs.update(runs.routine.SourceCluster, []*infomodels.ResponseBackupState{done("bar")},
		time.Now().Add(time.Second), testRoutines{})
	require.Len(t, ended, 1, "the routine is gone")
}

// TestRuns_FindsJobsItDidNotStart checks that a running job in a routine's path that no run holds
// opens a run of its own, which the next such job of its type joins, while ended jobs and jobs
// outside every routine's path are left alone.
func TestRuns_FindsJobsItDidNotStart(t *testing.T) {
	t.Parallel()

	logs := &recordedLogs{}
	runs := newTestRuns()
	runs.logger = slog.New(logs)
	elsewhere := job("elsewhere", "bar", infomodels.BackupStateBaseScanActive)
	elsewhere.Path = "abs/other/"
	incremental := job("incr", "bar", infomodels.BackupStateInit)
	incremental.ModifiedAfter = "1763980000"

	ended, _ := runs.list(at("bar", 50, 1), job("old", "bar", infomodels.BackupStateComplete), elsewhere)
	assert.Empty(t, ended)
	ended, _ = runs.list(at("bar", 50, 1), at("baz", 10, 1), incremental)
	assert.Empty(t, ended)

	state := runs.state()
	require.NotNil(t, state.Full)
	assert.Equal(t, []string{"job-bar", "job-baz"}, jobIDs(state.Full.ServerJobs))
	require.NotNil(t, state.Incremental)
	assert.Equal(t, []string{"incr"}, jobIDs(state.Incremental.ServerJobs))
	assert.Equal(t, 2, logs.count(slog.LevelInfo), "one log per run found")

	ended, _ = runs.list(done("bar"), done("baz"), incremental)
	require.Len(t, ended, 1)
	assert.Equal(t, model.BackupTypeFull, ended[0].backupType)
	require.NoError(t, ended[0].outcome())
}

// TestRuns_RunsEndOnTheirOwn checks that the runs of one routine and type end each with its own
// jobs: two starts, as concurrent incremental backups make, and jobs this process did not start.
func TestRuns_RunsEndOnTheirOwn(t *testing.T) {
	t.Parallel()

	runs := newTestRuns()
	runs.started(model.BackupTypeFull, "bar")
	runs.started(model.BackupTypeFull, "baz")
	found := job("found", "qux", infomodels.BackupStateBaseScanActive)
	ended, _ := runs.list(at("bar", 10, 1), at("baz", 10, 1), found)
	require.Empty(t, ended)
	assert.Len(t, runs.state().Full.ServerJobs, 3, "the runs of a type show as one backup")

	ended, _ = runs.list(done("bar"), at("baz", 50, 1), found)
	require.Len(t, ended, 1)
	assert.Equal(t, []string{"job-bar"}, jobIDs(ended[0].serverJobs()))

	ended, _ = runs.list(done("bar"), done("baz"), job("found", "qux", infomodels.BackupStateComplete))
	require.Len(t, ended, 2)
	assert.Nil(t, runs.state().Full)
}

// TestRuns_Cancel checks that canceling a routine wakes the poller and has its runs' jobs aborted:
// each that runs and is not being aborted, and each the run gains later.
func TestRuns_Cancel(t *testing.T) {
	t.Parallel()

	runs := newTestRuns()
	runs.started(model.BackupTypeFull, "bar", "baz")
	starting := runs.open(runs.routine, model.BackupTypeIncremental, time.Now())

	runs.Cancel("daily")

	select {
	case <-runs.cancels():
	default:
		t.Fatal("the cancel did not wake the poller")
	}

	_, aborts := runs.list(job("job-bar", "bar", infomodels.BackupStateAborting), at("baz", 10, 1))
	assert.Equal(t, []string{"job-baz"}, aborts)

	require.NoError(t, runs.close(starting, []startedJob{{namespace: "bar", id: "job-incr"}}, nil))
	operatorAborted := job("job-bar", "bar", infomodels.BackupStateAborted)
	operatorAborted.AbortOrigin = infomodels.AbortOriginOperator
	ended, aborts := runs.list(operatorAborted, done("baz"))
	assert.Equal(t, []string{"job-incr"}, aborts, "a job the canceled run started later")
	require.Len(t, ended, 1)
	require.ErrorIs(t, ended[0].outcome(), context.Canceled)
}

// TestStartRulesCountRuns checks, with the start rules and the registry of the service, that a run
// this process did not start makes a start of its type skip with no reservation held, and that the
// start is admitted once the run ends.
func TestStartRulesCountRuns(t *testing.T) {
	t.Parallel()

	runs := newTestRuns()
	history := service.NewMockHistoryManager(gomock.NewController(t))
	history.EXPECT().FindLastRun(gomock.Any(), runs.routine).
		Return(model.NewFullBackupTime(time.Now().Add(-time.Hour)), nil).AnyTimes()
	registry := service.NewBackupStateRegistry(history, testRoutines{runs.routine.Name: runs.routine}, runs.Runs)
	registry.Start(t.Context())
	registry.RequestHistorySync([]string{runs.routine.Name})
	startController := service.NewStartController(registry, service.NewStartDecider())

	runs.list(at("bar", 10, 1))
	_, err := startController.TryStart(runs.routine, time.Now(), model.BackupTypeFull)
	require.ErrorIs(t, err, service.ErrBackupSkipped, "a backup runs")

	runs.list(done("bar"))
	release, err := startController.TryStart(runs.routine, time.Now(), model.BackupTypeFull)
	require.NoError(t, err)
	release()
}

// TestRuns_AwaitListing checks that a start waits for the first listing of its cluster, and no
// longer than firstListingWait.
func TestRuns_AwaitListing(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		runs := newTestRuns()
		start := time.Now()

		runs.awaitListing(t.Context(), runs.routine.SourceCluster)
		assert.Equal(t, firstListingWait, time.Since(start), "never listed")

		go func() {
			time.Sleep(time.Second)
			runs.list()
		}()
		runs.awaitListing(t.Context(), runs.routine.SourceCluster)
		assert.Equal(t, firstListingWait+time.Second, time.Since(start), "listed a second later")

		runs.awaitListing(t.Context(), runs.routine.SourceCluster)
		assert.Equal(t, firstListingWait+time.Second, time.Since(start), "already listed")
	})
}
