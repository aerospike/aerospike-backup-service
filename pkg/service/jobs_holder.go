package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/prometheus"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/backup-go/models"
)

// restoreJob encapsulates the state and details of a single restore operation.
// A restore job can consist multiple backups (one full backup and several incremental) across multiple namespaces.
type restoreJob struct {
	sync.RWMutex

	// Volatile fields, should only be accessed under lock:

	// handlers contains the restore handlers of the job. Each follows one restore that the
	// job's backup mode has started.
	handlers []RestoreHandler

	// status indicates the current state of the job (Running, Done, Failed, Canceled).
	status model.RestoreState

	// err holds all errors that occurred during the job execution.
	// It is nil if the job is running or completed successfully.
	err error

	// totalRecords is the aggregated count of records to be restored across all backups.
	totalRecords uint64

	// finished is the timestamp marking the completion of the restore job.
	// It is nil for jobs that are still running.
	finished *time.Time

	// Immutable fields, set at creation:

	// started is the timestamp marking the initiation of the restore job.
	started time.Time

	// label provides a user-friendly name for the restore job, used for metrics.
	label string

	// cancel is a function that cancels the context of the restore job,
	// effectively stopping the operation.
	cancel context.CancelFunc
}

func newRestoreJob(label string, cancel context.CancelFunc) *restoreJob {
	return &restoreJob{
		status:  model.RestoreRunning,
		started: time.Now().Truncate(time.Millisecond),
		label:   label,
		cancel:  cancel,
	}
}

// addHandler adds a restore handler to the job.
func (j *restoreJob) addHandler(handler RestoreHandler) {
	j.Lock()
	defer j.Unlock()

	j.handlers = append(j.handlers, handler)
}

// addTotalRecords adds to the job's total record count.
func (j *restoreJob) addTotalRecords(t uint64) {
	j.Lock()
	defer j.Unlock()

	j.totalRecords += t
}

// finish marks the job as finished with a given status and error.
func (j *restoreJob) finish(err error, logger *slog.Logger) {
	j.Lock()

	j.finished = new(time.Now().Truncate(time.Millisecond))
	j.err = err

	switch {
	case err == nil:
		j.status = model.RestoreSuccess
		logger.Info("restore finished")
	case errors.Is(err, context.Canceled):
		j.status = model.RestoreCanceled
		logger.Info("restore canceled")
	case errors.Is(err, ErrRestorePrerequisitesFailed):
		j.status = model.RestoreFailure
		logger.Warn("failed to start restore", attr.Error(err))
	default:
		j.status = model.RestoreFailure
		logger.Error("restore failed", attr.Error(err))
	}

	finalStatus := j.status
	j.Unlock()

	prometheus.ObserveRestoreCompletion(finalStatus)
}

// buildStatus constructs a model.RestoreJobStatus from the current job state.
func (j *restoreJob) buildStatus() *model.RestoreJobStatus {
	j.RLock()
	defer j.RUnlock()

	run := sumRestoreRuns(j.handlers)
	runningJob := NewRestoreRunningJob(
		j.started, j.finished, run.Counters.ReadRecords, j.totalRecords, run.Metrics, j.status)

	return &model.RestoreJobStatus{
		Status:         j.status,
		Error:          j.err,
		Counters:       run.Counters,
		CurrentRestore: runningJob,
	}
}

// sumRestoreRuns adds up the counters and the throughput of the runs of a job's handlers.
func sumRestoreRuns(handlers []RestoreHandler) model.RestoreRun {
	var counters model.RestoreCounters

	metrics := make([]*models.Metrics, 0, len(handlers))
	for _, handler := range handlers {
		run := handler.Status()
		counters = counters.Plus(run.Counters)
		metrics = append(metrics, run.Metrics)
	}

	return model.RestoreRun{Counters: counters, Metrics: models.SumMetrics(metrics...)}
}

func (j *restoreJob) getStatus() model.RestoreState {
	j.RLock()
	defer j.RUnlock()
	return j.status
}

// RestoreJobsHolder is a thread-safe map of restore jobs. A backup mode's restore manager
// registers its jobs here and embeds the holder for the status, cancel and listing methods, which
// are the same in every mode. The holder also owns the lifetime of the jobs. Start hands it the
// service context and every job runs in a child of that context, so canceling the service cancels
// the jobs with it.
type RestoreJobsHolder struct {
	*collections.SafeMap[model.RestoreJobID, *restoreJob]

	mu sync.Mutex
	// lifetime is the parent of every job context. A restore outlives the HTTP request that
	// starts it, so its parent must be the service lifetime rather than the request's, and
	// this holder is the one owner of that lifetime. The mutex is here because Start writes
	// it while requests read it, not because a request could arrive first.
	lifetime context.Context //nolint:containedctx // the holder's purpose is job lifetimes; see above.
}

type JobNotFoundError struct {
	JobID model.RestoreJobID
}

func (e *JobNotFoundError) Error() string {
	return fmt.Sprintf("restore job with ID %d not found", e.JobID)
}

func NewJobNotFoundError(id model.RestoreJobID) *JobNotFoundError {
	return &JobNotFoundError{JobID: id}
}

// ErrRestorePrerequisitesFailed is wrapped by a restore that was refused before it started. The
// job is then logged as a warning, not as an error.
var ErrRestorePrerequisitesFailed = errors.New("restore pre-requisites failed")

// maxJSONSafeJobID bounds restore job ids to 2^53-1, the largest integer an IEEE-754 double
// holds exactly. Ids are written as bare JSON numbers, and clients that decode numbers as
// doubles (JavaScript, Swagger UI) would round anything larger into an id that does not exist.
const maxJSONSafeJobID = 1<<53 - 1

// NewRestoreJobsHolder returns a new RestoreJobsHolder. It is usable once Start has given
// it a lifetime, which Components.Start does before anything can reach it.
func NewRestoreJobsHolder() *RestoreJobsHolder {
	return &RestoreJobsHolder{
		SafeMap: collections.NewSafeMap[model.RestoreJobID, *restoreJob](),
	}
}

// Start makes ctx the parent of every restore job started from now on. Nothing is
// launched here: Start is the call every component gets the moment the service begins
// running, and this one needs that moment only to learn the run context.
func (h *RestoreJobsHolder) Start(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.lifetime = ctx
}

// NewJob registers a new restore job and returns its id together with the context the job
// must run in. The job is finished, and its context released, by FinishJob.
func (h *RestoreJobsHolder) NewJob(label string) (model.RestoreJobID, context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ctx, cancel := context.WithCancel(h.lifetime)
	// #nosec G404
	id := model.RestoreJobID(rand.Int64N(maxJSONSafeJobID) + 1)
	h.Store(id, newRestoreJob(label, cancel))

	return id, ctx
}

// AddHandler adds the handler of a restore the job has started, so that the job's status includes it.
func (h *RestoreJobsHolder) AddHandler(id model.RestoreJobID, handler RestoreHandler) {
	if job, ok := h.Load(id); ok {
		job.addHandler(handler)
	}
}

// AddTotalRecords adds the record count of backups the job restores: once per backup for
// restore by time, once for the backup of a restore by path.
func (h *RestoreJobsHolder) AddTotalRecords(id model.RestoreJobID, t uint64) {
	if job, ok := h.Load(id); ok {
		job.addTotalRecords(t)
	}
}

// FinishJob records the outcome of a job started by NewJob and releases its context, which
// also stops any of its sub-operations that are still running.
func (h *RestoreJobsHolder) FinishJob(id model.RestoreJobID, err error, logger *slog.Logger) {
	if job, ok := h.Load(id); ok {
		job.cancel()
		job.finish(err, logger)
	}
}

// StatusCounts returns counts of restore jobs by status.
func (h *RestoreJobsHolder) StatusCounts() map[model.RestoreState]int {
	counts := make(map[model.RestoreState]int)

	h.Iterate(func(_ model.RestoreJobID, job *restoreJob) {
		counts[job.getStatus()]++
	})

	return counts
}

// JobStatus returns the status of the job with the given id.
func (h *RestoreJobsHolder) JobStatus(jobID model.RestoreJobID) (*model.RestoreJobStatus, error) {
	job, err := h.getJob(jobID)
	if err != nil {
		return nil, err
	}

	return job.buildStatus(), nil
}

// CancelRestore cancels an ongoing restore.
func (h *RestoreJobsHolder) CancelRestore(jobID model.RestoreJobID) error {
	job, err := h.getJob(jobID)
	if err != nil {
		return err
	}

	job.cancel()

	return nil
}

// GetFilteredJobs returns all jobs matching the given filters as a map of jobId -> RestoreJobStatus.
func (h *RestoreJobsHolder) GetFilteredJobs(
	timeBounds model.TimeBounds,
	statusFilter model.StatusFilter,
) map[model.RestoreJobID]*model.RestoreJobStatus {
	results := make(map[model.RestoreJobID]*model.RestoreJobStatus)

	h.Iterate(func(id model.RestoreJobID, job *restoreJob) {
		if !timeBounds.Contains(job.started) {
			return
		}

		// Build the status first to get a consistent snapshot of the job.
		// This prevents a race condition where the job's status changes
		// between filtering and building the result.
		status := job.buildStatus()

		// Now, filter based on the consistent snapshot.
		if !statusFilter.Matches(status.Status) {
			return
		}

		results[id] = status
	})

	return results
}

func (h *RestoreJobsHolder) getJob(id model.RestoreJobID) (*restoreJob, error) {
	if job, exists := h.Load(id); exists {
		return job, nil
	}

	return nil, NewJobNotFoundError(id)
}
