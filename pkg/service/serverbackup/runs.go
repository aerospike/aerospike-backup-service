package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/backup-go/models"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

const (
	// lostAfterPolls is how many listings in a row may leave a job out before it counts as lost.
	lostAfterPolls = 2
	// firstListingWait bounds how long a start waits for the first listing of its cluster.
	firstListingWait = 2 * jobPollInterval
)

// Runs holds the routines' server backups that have not ended: those this process started, and
// those the listings show in a routine's path that it did not start. It is plain state: a start
// opens and closes its run, each listing of a cluster updates the runs on it, and the routine
// state reads them. The servers keep the jobs, so after a restart the first listings rebuild it.
type Runs struct {
	logger *slog.Logger
	// wake is signaled when a routine is canceled, so the poller aborts its jobs at once.
	wake chan struct{}

	mu   sync.Mutex
	runs []*run
	// last holds the server jobs of each routine's last ended run of each type.
	last map[routineType][]model.ServerBackupJob
	// listed holds, by cluster hash, a channel the cluster's first successful listing closes.
	listed map[uint64]chan struct{}
}

// routineType names a routine's backups of one type.
type routineType struct {
	routine    string
	backupType model.BackupType
}

// run is a routine's server backup of one type: the jobs one start began, one per namespace, or
// the jobs in the routine's path that this process found running and did not start.
type run struct {
	// routine is the run's routine as the configuration last had it.
	routine    *model.BackupRoutine
	backupType model.BackupType
	start      time.Time
	// starting is set while the start that opened the run starts its jobs: the run cannot end yet.
	starting bool
	// found is set on a run of jobs this process did not start.
	found    bool
	canceled bool
	jobs     []*runJob
	// startErr joins the errors of the namespaces whose job did not start.
	startErr error
}

// runJob is a server backup job of a run.
type runJob struct {
	id        string
	namespace string
	// learned is when this process learned of the job: a listing that began earlier may lack it.
	learned time.Time
	// status is the job as last listed, nil until it is listed.
	status *infomodels.ResponseBackupState
	// missed counts the listings in a row that left the job out.
	missed int
}

var _ service.RunningBackups = (*Runs)(nil)

// startedJob is a job a start began.
type startedJob struct {
	namespace string
	id        string
}

// NewRuns returns an empty Runs.
func NewRuns() *Runs {
	return &Runs{
		logger: slog.Default(),
		wake:   make(chan struct{}, 1),
		last:   make(map[routineType][]model.ServerBackupJob),
		listed: make(map[uint64]chan struct{}),
	}
}

// awaitListing waits for the first successful listing of cluster, at most firstListingWait, so that
// a start sees the backups that already run on it.
func (r *Runs) awaitListing(ctx context.Context, cluster *model.AerospikeCluster) {
	r.mu.Lock()
	listed := r.listedLocked(cluster.Hash())
	r.mu.Unlock()

	timer := time.NewTimer(firstListingWait)
	defer timer.Stop()

	select {
	case <-listed:
	case <-timer.C:
	case <-ctx.Done():
	}
}

// cancels signals each cancel of a routine.
func (r *Runs) cancels() <-chan struct{} {
	return r.wake
}

// open opens the run of a start of the routine's backup at start, before the start begins any
// job.
func (r *Runs) open(routine *model.BackupRoutine, backupType model.BackupType, start time.Time) *run {
	r.mu.Lock()
	defer r.mu.Unlock()

	opened := &run{routine: routine, backupType: backupType, start: start, starting: true}
	r.runs = append(r.runs, opened)

	return opened
}

// close ends the start of opened: it adds the jobs the start began, in its namespaces' order, and
// the errors of those that did not start. A run that began no job ends here: close forgets it and
// returns startErr.
func (r *Runs) close(opened *run, jobs []startedJob, startErr error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	opened.starting = false
	opened.startErr = startErr
	if len(jobs) == 0 {
		r.forgetLocked(opened)
		return startErr
	}

	now := time.Now()
	for _, job := range jobs {
		opened.jobs = append(opened.jobs, &runJob{id: job.id, namespace: job.namespace, learned: now})
	}

	return nil
}

// Cancel cancels the routine's runs. The poller aborts their jobs at once, and each job they gain
// later.
func (r *Runs) Cancel(routineName string) {
	r.mu.Lock()
	for _, run := range r.runs {
		if run.routine.Name == routineName {
			run.canceled = true
		}
	}
	r.mu.Unlock()

	select {
	case r.wake <- struct{}{}:
	default: // a wake is already due.
	}
}

// Describe sets in state the routine's running backups and the server jobs of its last ended
// ones. The runs of a type show as one backup.
func (r *Runs) Describe(routineName string, state *model.RoutineState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state.Full = r.runningLocked(routineType{routineName, model.BackupTypeFull})
	state.Incremental = r.runningLocked(routineType{routineName, model.BackupTypeIncremental})
	state.LastFullJobs = r.last[routineType{routineName, model.BackupTypeFull}]
	state.LastIncrementalJobs = r.last[routineType{routineName, model.BackupTypeIncremental}]
}

// clusters returns the clusters of the runs, as routines has them.
func (r *Runs) clusters(routines map[string]*model.BackupRoutine) []*model.AerospikeCluster {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refreshLocked(routines)
	clusters := make([]*model.AerospikeCluster, 0, len(r.runs))
	for _, run := range r.runs {
		clusters = append(clusters, run.routine.SourceCluster)
	}

	return clusters
}

// update follows the runs on cluster through a listing of its jobs that began at at. It takes on
// each running job in a routine's path that no run holds, unless a start of that routine is
// starting: the start may have begun it. It returns the runs that ended, which it forgets, and the
// jobs of canceled runs to abort.
func (r *Runs) update(
	cluster *model.AerospikeCluster,
	jobs []*infomodels.ResponseBackupState,
	at time.Time,
	routines map[string]*model.BackupRoutine,
) (ended []*run, aborts []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refreshLocked(routines)
	hash := cluster.Hash()
	listed := jobsByID(jobs)
	for _, run := range r.runs {
		if run.routine.SourceCluster.Hash() == hash {
			run.follow(listed, at)
		}
	}
	for _, job := range jobs {
		r.findLocked(job, cluster, at, routines)
	}

	kept := r.runs[:0]
	for _, run := range r.runs {
		switch {
		case run.routine.SourceCluster.Hash() != hash:
		case run.ended():
			ended = append(ended, run)
			r.last[routineType{run.routine.Name, run.backupType}] = run.serverJobs()
			continue
		case run.canceled:
			aborts = append(aborts, run.abortable()...)
		}
		kept = append(kept, run)
	}
	clear(r.runs[len(kept):])
	r.runs = kept

	closeOnce(r.listedLocked(hash))

	return ended, aborts
}

// findLocked takes on job when it runs in a routine's path, no run holds it and no start of the
// routine is starting: it joins the run of the routine's jobs of its type that this process found,
// or opens one. The caller holds mu.
func (r *Runs) findLocked(
	job *infomodels.ResponseBackupState,
	cluster *model.AerospikeCluster,
	at time.Time,
	routines map[string]*model.BackupRoutine,
) {
	if jobEnded(job.State) || r.holdsLocked(job.JobID) {
		return
	}

	routine := routineOf(job, cluster, routines)
	if routine == nil {
		return
	}

	// A start's job may look like the other type: an incremental backup of a namespace with no
	// backup to build on has no lower bound.
	if r.startingLocked(routine.Name) {
		return // the next listing tells whether the start began it.
	}

	backupType := jobType(job)
	start := startOf(job, at)
	found := r.foundLocked(routine.Name, backupType)
	if found == nil {
		found = &run{routine: routine, backupType: backupType, start: start, found: true}
		r.runs = append(r.runs, found)
		r.logger.Info("Following a backup this service did not start",
			attr.Routine(routine.Name), slog.String("type", string(backupType)), slog.Time("startTime", start))
	}

	if start.Before(found.start) {
		found.start = start
	}
	found.jobs = append(found.jobs, &runJob{id: job.JobID, namespace: job.Namespace, learned: at, status: job})
}

// refreshLocked gives each run its routine as routines has it, so that a run follows an edited
// routine or cluster. A run whose routine is gone keeps the one it had. The caller holds mu.
func (r *Runs) refreshLocked(routines map[string]*model.BackupRoutine) {
	for _, run := range r.runs {
		if routine, ok := routines[run.routine.Name]; ok {
			run.routine = routine
		}
	}
}

func (r *Runs) holdsLocked(jobID string) bool {
	for _, run := range r.runs {
		for _, job := range run.jobs {
			if job.id == jobID {
				return true
			}
		}
	}

	return false
}

func (r *Runs) foundLocked(routineName string, backupType model.BackupType) *run {
	for _, run := range r.runs {
		if run.found && run.routine.Name == routineName && run.backupType == backupType {
			return run
		}
	}

	return nil
}

func (r *Runs) startingLocked(routineName string) bool {
	for _, run := range r.runs {
		if run.starting && run.routine.Name == routineName {
			return true
		}
	}

	return false
}

// runningLocked is the backup the runs of a routine and type make, or nil if it has none. Every job
// weighs the same: an ended one counts whole and a running one by its progress. The throughput is
// the running jobs' sum. The caller holds mu.
func (r *Runs) runningLocked(key routineType) *model.RunningJob {
	var (
		start   time.Time
		jobs    []*runJob
		running bool
	)
	for _, run := range r.runs {
		if run.routine.Name != key.routine || run.backupType != key.backupType {
			continue
		}
		if !running || run.start.Before(start) {
			start = run.start
		}
		running = true
		jobs = append(jobs, run.jobs...)
	}
	if !running {
		return nil
	}

	var done float64
	var metrics models.Metrics
	serverJobs := make([]model.ServerBackupJob, 0, len(jobs))
	for _, job := range jobs {
		if job.status != nil {
			serverJobs = append(serverJobs, newServerBackupJob(job.status))
		}

		switch {
		case job.ended():
			done++
		case job.status != nil:
			done += job.status.ProgressPct / 100
			metrics.RecordsPerSecond += job.status.RecsPerSec
			metrics.KilobytesPerSecond += job.status.BytesOutPerSec / 1024
		}
	}

	var progress float64
	if len(jobs) > 0 { // a start that has not begun its jobs yet has none.
		progress = done / float64(len(jobs))
	}

	return service.NewBackupRunningJob(model.BackupRun{
		StartTime: start,
		Progress:  progress,
		Server:    &model.ServerProgress{Jobs: serverJobs, Metrics: &metrics},
	})
}

// listedLocked returns the channel the first successful listing of the cluster closes. The caller
// holds mu.
func (r *Runs) listedLocked(hash uint64) chan struct{} {
	listed, ok := r.listed[hash]
	if !ok {
		listed = make(chan struct{})
		r.listed[hash] = listed
	}

	return listed
}

func (r *Runs) forgetLocked(forgotten *run) {
	for i, run := range r.runs {
		if run == forgotten {
			r.runs = append(r.runs[:i], r.runs[i+1:]...)
			return
		}
	}
}

// follow records what a listing that began at at shows of the run's jobs that have not ended.
func (r *run) follow(listed map[string]*infomodels.ResponseBackupState, at time.Time) {
	for _, job := range r.jobs {
		if job.ended() {
			continue
		}

		if status, ok := listed[job.id]; ok {
			job.status, job.missed = status, 0
		} else if at.After(job.learned) {
			job.missed++
		}
	}
}

// ended reports whether every job of the run has ended, and its start has begun them all.
func (r *run) ended() bool {
	if r.starting {
		return false
	}

	for _, job := range r.jobs {
		if !job.ended() {
			return false
		}
	}

	return true
}

// abortable returns the jobs of the run that may still run and are not being aborted.
func (r *run) abortable() []string {
	var jobIDs []string
	for _, job := range r.jobs {
		if !job.ended() && (job.status == nil || job.status.State != infomodels.BackupStateAborting) {
			jobIDs = append(jobIDs, job.id)
		}
	}

	return jobIDs
}

// outcome is the error the run ended with: the namespaces whose job did not start, and each job that
// did not complete. A job an operator aborted, through this service or not, makes it canceled.
func (r *run) outcome() error {
	errs := []error{r.startErr}
	for _, job := range r.jobs {
		if err := job.err(); err != nil {
			errs = append(errs, fmt.Errorf("namespace %s: %w", job.namespace, err))
		}
	}

	return errors.Join(errs...)
}

// serverJobs returns the run's jobs that have been listed, each as last listed.
func (r *run) serverJobs() []model.ServerBackupJob {
	jobs := make([]model.ServerBackupJob, 0, len(r.jobs))
	for _, job := range r.jobs {
		if job.status != nil {
			jobs = append(jobs, newServerBackupJob(job.status))
		}
	}

	return jobs
}

// warnUnread warns of each job the server completed without the records it could not read: the
// backup lacks them.
func (r *run) warnUnread(logger *slog.Logger) {
	for _, job := range r.jobs {
		if job.status != nil && job.status.State == infomodels.BackupStateComplete && job.status.RecsReadFailed > 0 {
			logger.Warn("Server backup completed without the records the server could not read",
				slog.String("jobId", job.id),
				slog.String("namespace", job.namespace),
				slog.Uint64("recordsReadFailed", job.status.RecsReadFailed))
		}
	}
}

func (j *runJob) lost() bool {
	return j.missed >= lostAfterPolls
}

func (j *runJob) ended() bool {
	return j.lost() || (j.status != nil && jobEnded(j.status.State))
}

// err is the error the job ended with, nil if it completed.
func (j *runJob) err() error {
	if j.lost() {
		return fmt.Errorf("%s %s is lost: the server does not list it", backupJobKind, j.id)
	}

	switch j.status.State {
	case infomodels.BackupStateFailed:
		return errJobEnded(backupJobKind, j.id, "failed", j.status.AbortOrigin, j.status.ErrorReason)
	case infomodels.BackupStateAborted:
		err := errJobEnded(backupJobKind, j.id, "was aborted", j.status.AbortOrigin, j.status.ErrorReason)
		if j.status.AbortOrigin == infomodels.AbortOriginOperator {
			return fmt.Errorf("%w: %w", context.Canceled, err)
		}

		return err
	default:
		return nil
	}
}

// startOf returns when job started, or at if it has not started yet.
func startOf(job *infomodels.ResponseBackupState, at time.Time) time.Time {
	if job.StartTime.IsZero() {
		return at
	}

	return job.StartTime
}

func closeOnce(ch chan struct{}) {
	select {
	case <-ch:
	default:
		close(ch)
	}
}
