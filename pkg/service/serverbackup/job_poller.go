package serverbackup

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// jobPollInterval is how often the poller lists the backup jobs of each cluster.
const jobPollInterval = 5 * time.Second

// routineProvider supplies the configured backup routines, read anew on every poll.
type routineProvider interface {
	Routines() map[string]*model.BackupRoutine
}

// jobPoller lists the backup jobs of each cluster a routine backs up or a run is on, every
// jobPollInterval, whoever started them. It hands each listing to the runs, aborts the jobs of the
// canceled ones, and reports each run that ended. It logs each job that appears or changes state,
// with the routine whose path it writes to.
type jobPoller struct {
	routines      routineProvider
	clientManager aerospike.ClientManager
	runs          *Runs
	completion    service.BackupCompletionHandler
	reporter      service.BackupReporter
	logger        *slog.Logger
	// listings holds the last successful listing of each cluster polled, by the cluster's hash. Only
	// the poll uses it.
	listings map[uint64]map[string]*infomodels.ResponseBackupState
}

var _ service.ServerJobPoller = (*jobPoller)(nil)

// NewJobPoller returns the ServerJobPoller of the clusters of routines and of runs. It reports
// each run that ends through reporter, and completes it through completion.
func NewJobPoller(
	routines routineProvider,
	clientManager aerospike.ClientManager,
	runs *Runs,
	completion service.BackupCompletionHandler,
	reporter service.BackupReporter,
) service.ServerJobPoller {
	return &jobPoller{
		routines:      routines,
		clientManager: clientManager,
		runs:          runs,
		completion:    completion,
		reporter:      reporter,
		logger:        slog.Default(),
		listings:      make(map[uint64]map[string]*infomodels.ResponseBackupState),
	}
}

// Start polls at once, then every jobPollInterval and whenever a routine is canceled, until ctx
// is canceled.
func (p *jobPoller) Start(ctx context.Context) {
	go p.run(ctx)
}

// run polls until ctx is canceled. A poll that outlasts the interval delays the next one rather
// than overlapping it.
func (p *jobPoller) run(ctx context.Context) {
	ticker := time.NewTicker(jobPollInterval)
	defer ticker.Stop()

	for {
		p.poll(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-p.runs.cancels():
		}
	}
}

// poll lists the jobs of each cluster to poll, and forgets the listings of the others.
func (p *jobPoller) poll(ctx context.Context) {
	routines := p.routines.Routines()

	clusters := make(map[uint64]*model.AerospikeCluster)
	for _, routine := range routines {
		clusters[routine.SourceCluster.Hash()] = routine.SourceCluster
	}
	for _, cluster := range p.runs.clusters(routines) {
		clusters[cluster.Hash()] = cluster
	}

	for _, cluster := range clusters {
		p.pollCluster(ctx, cluster, routines)
	}

	maps.DeleteFunc(p.listings, func(hash uint64, _ map[string]*infomodels.ResponseBackupState) bool {
		_, polled := clusters[hash]
		return !polled
	})
}

// pollCluster lists the jobs of cluster, bounded by jobPollInterval, and hands the listing to the
// runs. It aborts the jobs of the canceled runs on cluster and reports the runs that ended. A
// failed listing changes nothing.
func (p *jobPoller) pollCluster(
	ctx context.Context,
	cluster *model.AerospikeCluster,
	routines map[string]*model.BackupRoutine,
) {
	at := time.Now()
	listCtx, cancel := context.WithTimeout(ctx, jobPollInterval)
	defer cancel()

	client, err := p.clientManager.GetClient(listCtx, cluster, nil, p.logger)
	if err != nil {
		p.logger.Warn("Failed to list server backup jobs",
			slog.String("cluster", cluster.ClusterLabel), attr.Error(err))
		return
	}
	defer p.clientManager.Close(client)

	jobs, err := client.InfoClient().ListBackupJobs(listCtx)
	if err != nil {
		p.logger.Warn("Failed to list server backup jobs",
			slog.String("cluster", cluster.ClusterLabel), attr.Error(err))
		return
	}

	p.report(cluster, jobs, routines)
	ended, aborts := p.runs.update(cluster, jobs, at, routines)

	for _, jobID := range aborts {
		if err := client.InfoClient().AbortBackup(listCtx, jobID); err != nil {
			p.logger.Warn("Failed to abort server backup", slog.String("jobId", jobID), attr.Error(err))
		}
	}

	for _, run := range ended {
		p.finish(ctx, run)
	}
}

// finish reports a run that ended, after the completion hooks of its outcome: on success the
// history rescan and, after a full backup, retention.
func (p *jobPoller) finish(ctx context.Context, ended *run) {
	logger := p.logger.With(attr.Routine(ended.routine.Name))
	ended.warnUnread(logger)

	err := ended.outcome()
	if err == nil {
		p.completion.OnSuccess(ctx, ended.routine, ended.backupType, ended.start, logger)
		p.reporter.Report(ended.routine.Name, ended.backupType, time.Since(ended.start), nil, logger)

		return
	}

	p.completion.OnFailure(ended.routine, ended.backupType)
	p.reporter.Report(ended.routine.Name, ended.backupType, time.Since(ended.start),
		fmt.Errorf("%s backup failed: %w", ended.backupType, err), logger)
}

// report logs each job of cluster that is new since the cluster's previous listing or changed its
// state. On a cluster's first listing the jobs that already ended are not logged: each node keeps
// up to 100 of them.
func (p *jobPoller) report(
	cluster *model.AerospikeCluster,
	jobs []*infomodels.ResponseBackupState,
	routines map[string]*model.BackupRoutine,
) {
	hash := cluster.Hash()
	previous := p.listings[hash]
	p.listings[hash] = jobsByID(jobs)

	for _, job := range jobs {
		before, known := previous[job.JobID]
		if (known && before.State == job.State) || (previous == nil && jobEnded(job.State)) {
			continue
		}

		p.logger.Info("Server backup job",
			slog.String("cluster", cluster.ClusterLabel),
			slog.String("jobId", job.JobID),
			slog.String("namespace", job.Namespace),
			slog.String("state", string(job.State)),
			slog.String("type", string(jobType(job))),
			slog.String("bucket", job.S3Bucket),
			slog.String("path", job.Path),
			attr.Routine(nameOf(routineOf(job, cluster, routines))))
	}
}

func jobsByID(jobs []*infomodels.ResponseBackupState) map[string]*infomodels.ResponseBackupState {
	byID := make(map[string]*infomodels.ResponseBackupState, len(jobs))
	for _, job := range jobs {
		byID[job.JobID] = job
	}

	return byID
}

// routineOf returns the routine of cluster whose bucket and path job writes to, or nil if job
// writes outside every routine's path. The server ends the path it reports with "/".
func routineOf(
	job *infomodels.ResponseBackupState,
	cluster *model.AerospikeCluster,
	routines map[string]*model.BackupRoutine,
) *model.BackupRoutine {
	for _, routine := range routines {
		storage, ok := routine.Storage.(*model.S3Storage)
		if ok && routine.SourceCluster.Hash() == cluster.Hash() && storage.Bucket == job.S3Bucket &&
			routinePath(storage, routine.Name)+"/" == job.Path {
			return routine
		}
	}

	return nil
}

// nameOf returns the routine's name, or "" for none.
func nameOf(routine *model.BackupRoutine) string {
	if routine == nil {
		return ""
	}

	return routine.Name
}

// jobType returns the type of backup job: one with a lower time bound is incremental.
func jobType(job *infomodels.ResponseBackupState) model.BackupType {
	if job.ModifiedAfter != "" {
		return model.BackupTypeIncremental
	}

	return model.BackupTypeFull
}

// jobEnded reports whether a backup job in state has ended.
func jobEnded(state infomodels.BackupState) bool {
	switch state {
	case infomodels.BackupStateComplete, infomodels.BackupStateFailed, infomodels.BackupStateAborted:
		return true
	default:
		return false
	}
}
