package service

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go/models"
)

// restoreJob encapsulates the state and details of a single restore operation.
// A restore job can consist multiple backups (one full backup and several incremental) across multiple namespaces.
type restoreJob struct {
	sync.RWMutex

	// Volatile fields, should only be accessed under lock:

	// handlers contains the list of active restore handlers from the underlying backup library.
	// Each handler corresponds to a specific backup being restored.
	handlers []restoreexecutor.RestoreHandler

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

// addHandler adds a new restore handler to the job.
func (j *restoreJob) addHandler(handler restoreexecutor.RestoreHandler) {
	j.Lock()
	defer j.Unlock()

	j.handlers = append(j.handlers, handler)
}

// addTotalRecords adds to the total records count for the job.
func (j *restoreJob) addTotalRecords(t uint64) {
	j.Lock()
	defer j.Unlock()

	j.totalRecords += t
}

// finish marks the job as finished with a given status and error.
func (j *restoreJob) finish(err error, logger *slog.Logger) {
	j.Lock()
	defer j.Unlock()

	j.finished = ptr.Of(time.Now().Truncate(time.Millisecond))
	j.err = err

	switch {
	case err == nil:
		j.status = model.RestoreDone
		logger.Info("restore finished")
	case errors.Is(err, context.Canceled):
		j.status = model.RestoreCanceled
		logger.Info("restore canceled")
	case errors.Is(err, ErrRestorePrerequisitesFailed):
		j.status = model.RestoreFailed
		logger.Warn("failed to start restore", attr.Error(err))
	default:
		j.status = model.RestoreFailed
		logger.Error("restore failed", attr.Error(err))
	}

	observeRestoreCompletion(j.status)
}

// buildStatus constructs a model.RestoreJobStatus from the current job state.
func (j *restoreJob) buildStatus() *model.RestoreJobStatus {
	j.RLock()
	defer j.RUnlock()

	metrics := make([]*models.Metrics, 0, len(j.handlers))
	stats := make([]*models.RestoreStats, 0, len(j.handlers))
	for _, handler := range j.handlers {
		metrics = append(metrics, handler.GetMetrics())
		stats = append(stats, handler.GetStats())
	}

	sumMetrics := models.SumMetrics(metrics...)
	restoreStats := models.SumRestoreStats(stats...)
	doneRecords := restoreStats.GetReadRecords()
	runningJob := NewRestoreRunningJob(
		j.started, j.finished, doneRecords, j.totalRecords, sumMetrics, j.status)

	return &model.RestoreJobStatus{
		Status:         j.status,
		Error:          j.err,
		Counters:       restoreStats,
		CurrentRestore: runningJob,
	}
}

func (j *restoreJob) getStatus() model.RestoreState {
	j.RLock()
	defer j.RUnlock()
	return j.status
}

// RestoreJobsHolder is a thread-safe map of restore jobs.
type RestoreJobsHolder struct {
	*collections.SafeMap[model.RestoreJobID, *restoreJob]
}

// NewRestoreJobsHolder returns a new RestoreJobsHolder.
func NewRestoreJobsHolder() *RestoreJobsHolder {
	return &RestoreJobsHolder{
		SafeMap: collections.NewSafeMap[model.RestoreJobID, *restoreJob](),
	}
}

// newJob creates a new restore job and return its id.
func (h *RestoreJobsHolder) newJob(label string, cancel context.CancelFunc) model.RestoreJobID {
	// #nosec G404
	id := model.RestoreJobID(rand.Int64())
	h.Store(id, newRestoreJob(label, cancel))

	return id
}

// addHandler should be called for each backup (full or incremental) handler.
func (h *RestoreJobsHolder) addHandler(id model.RestoreJobID, handler restoreexecutor.RestoreHandler) {
	if job, ok := h.Load(id); ok {
		job.addHandler(handler)
	}
}

// addTotalRecords should be called once for each namespace in the beginning
// of the restore process.
func (h *RestoreJobsHolder) addTotalRecords(id model.RestoreJobID, t uint64) {
	if job, ok := h.Load(id); ok {
		job.addTotalRecords(t)
	}
}

func (h *RestoreJobsHolder) finishJob(id model.RestoreJobID, err error, logger *slog.Logger) {
	if job, ok := h.Load(id); ok {
		job.finish(err, logger)
	}
}

// StatusCounts returns counts of restore jobs by status in one pass.
func (h *RestoreJobsHolder) StatusCounts() map[model.RestoreState]int {
	counts := make(map[model.RestoreState]int)

	h.Iterate(func(_ model.RestoreJobID, job *restoreJob) {
		counts[job.getStatus()]++
	})

	return counts
}

func (h *RestoreJobsHolder) getJob(id model.RestoreJobID) (*restoreJob, error) {
	if job, exists := h.Load(id); exists {
		return job, nil
	}

	return nil, NewErrJobNotFound(id)
}
