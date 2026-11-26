package serverbackup

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
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
		poller, _, _ := newTestPoller(gomock.NewController(t), func(string) ([]*infomodels.ResponseBackupState, error) {
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

	const (
		routinePrefix = "abs/daily/" // the path of routine "daily", as the server reports it.
		after         = "1763980000"
	)
	job := func(id string, state infomodels.BackupState, bucket, path, modifiedAfter string,
	) *infomodels.ResponseBackupState {
		return &infomodels.ResponseBackupState{
			JobID: id, Namespace: "bar", State: state, S3Bucket: bucket, Path: path, ModifiedAfter: modifiedAfter,
		}
	}
	polls := []struct {
		jobs []*infomodels.ResponseBackupState
		err  error
	}{
		{jobs: []*infomodels.ResponseBackupState{
			job("full", infomodels.BackupStateBaseScanActive, "bucket", routinePrefix, ""),
			job("old", infomodels.BackupStateComplete, "bucket", routinePrefix, ""),
			job("manual", infomodels.BackupStateBaseScanActive, "bucket", "elsewhere", ""),
		}},
		{err: errors.New("cluster down")},
		{jobs: []*infomodels.ResponseBackupState{
			job("full", infomodels.BackupStateComplete, "bucket", routinePrefix, ""),
			job("old", infomodels.BackupStateComplete, "bucket", routinePrefix, ""),
			job("manual", infomodels.BackupStateBaseScanActive, "bucket", "elsewhere", ""),
			job("incr", infomodels.BackupStateInit, "bucket", routinePrefix, after),
		}},
	}

	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		poller, logs, _ := newTestPoller(gomock.NewController(t), func(string) ([]*infomodels.ResponseBackupState, error) {
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
		}, logs.jobs())
		assert.Equal(t, 1, logs.count(slog.LevelWarn))
	})
}

// TestJobPoller_Next checks that a waiter gets the first successful poll that began after the time
// it gives: at once if that poll is already in, never one that failed.
func TestJobPoller_Next(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		// Poll n lists the job "n"; the second poll fails.
		poller, _, cluster := newTestPoller(gomock.NewController(t), func(string) ([]*infomodels.ResponseBackupState, error) {
			poll := calls.Add(1)
			if poll == 2 {
				return nil, errors.New("cluster down")
			}

			return []*infomodels.ResponseBackupState{{JobID: strconv.Itoa(int(poll))}}, nil
		})
		ctx, cancel := context.WithCancel(t.Context())
		poller.Start(ctx)
		synctest.Wait()
		firstPoll := time.Now()

		listing, err := poller.Next(ctx, cluster, firstPoll.Add(-time.Second))
		require.NoError(t, err)
		assert.Contains(t, listing.Jobs, "1")
		assert.Equal(t, firstPoll, time.Now(), "the first poll is already in")

		listing, err = poller.Next(ctx, cluster, firstPoll)
		require.NoError(t, err)
		assert.Contains(t, listing.Jobs, "3", "the first poll did not begin after, and the second failed")
		assert.Equal(t, firstPoll.Add(2*jobPollInterval), listing.At)

		cancel()
		synctest.Wait()
	})
}

// TestJobPoller_PollsClustersRunsWaitOn checks that a cluster no routine backs up is polled while
// a run waits on it, and only then.
func TestJobPoller_PollsClustersRunsWaitOn(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		polls := make(map[string]int)
		poll := func(cluster string) ([]*infomodels.ResponseBackupState, error) {
			mu.Lock()
			defer mu.Unlock()
			polls[cluster]++

			return []*infomodels.ResponseBackupState{}, nil
		}
		poller, _, _ := newTestPoller(gomock.NewController(t), poll)
		other := &model.AerospikeCluster{ClusterLabel: "other"}
		ctx, cancel := context.WithCancel(t.Context())
		poller.Start(ctx)
		synctest.Wait()

		waited := make(chan error)
		go func() {
			_, err := poller.Next(ctx, other, time.Now())
			waited <- err
		}()
		time.Sleep(jobPollInterval)
		require.NoError(t, <-waited)
		time.Sleep(jobPollInterval)
		synctest.Wait()
		cancel()
		synctest.Wait()

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, map[string]int{"source": 3, "other": 1}, polls)
	})
}

// newTestPoller returns a poller of one routine, "daily", on the cluster it returns. A poll of a
// cluster returns what list returns for the cluster's label.
func newTestPoller(
	ctrl *gomock.Controller,
	list func(cluster string) ([]*infomodels.ResponseBackupState, error),
) (*jobPoller, *recordedLogs, *model.AerospikeCluster) {
	routine := &model.BackupRoutine{
		Name:          "daily",
		SourceCluster: &model.AerospikeCluster{ClusterLabel: "source"},
		Storage:       &model.S3Storage{Bucket: "bucket", Path: "abs"},
	}
	// The poller polls one cluster at a time: polled is the one it took a client of last.
	var polled string
	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().ListBackupJobs(gomock.Any()).DoAndReturn(
		func(context.Context) ([]*infomodels.ResponseBackupState, error) { return list(polled) },
	).AnyTimes()
	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().InfoClient().Return(infoClient).AnyTimes()
	clientManager := aerospike.NewMockClientManager(ctrl)
	clientManager.EXPECT().GetClient(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, cluster *model.AerospikeCluster, _, _ any) (aerospike.Client, error) {
			polled = cluster.ClusterLabel
			return client, nil
		}).AnyTimes()
	clientManager.EXPECT().Close(client).AnyTimes()

	logs := &recordedLogs{}
	poller, _ := NewJobPoller(testRoutines{routine.Name: routine}, clientManager).(*jobPoller)
	poller.logger = slog.New(logs)

	return poller, logs, routine.SourceCluster
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
