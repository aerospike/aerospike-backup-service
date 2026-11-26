package serverbackup

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
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

// JobPoller lists the backup jobs of every cluster a routine backs up, every jobPollInterval,
// whoever started them, and hands each listing to the runs that follow jobs on that cluster.
type JobPoller interface {
	service.ServerJobPoller
	// Next waits for the first successful poll of cluster that began after after, and returns
	// its listing. It ends only with ctx. A cluster is polled while a run waits on it, even
	// when no routine backs it up any more.
	Next(ctx context.Context, cluster *model.AerospikeCluster, after time.Time) (JobListing, error)
}

// JobListing is what one poll of a cluster listed.
type JobListing struct {
	// Jobs holds every backup job the cluster's nodes know, by id.
	Jobs map[string]*infomodels.ResponseBackupState
	// At is when the poll began: a job started later may be missing from it.
	At time.Time
}

// jobPoller polls the clusters in turn. It logs each job that appears or changes state, with the
// routine whose path it writes to.
type jobPoller struct {
	routines      routineProvider
	clientManager aerospike.ClientManager
	logger        *slog.Logger

	mu sync.Mutex
	// clusters holds what the poller knows of each cluster, by the cluster's hash.
	clusters map[uint64]*clusterPolls
}

// clusterPolls is what the poller knows of one cluster.
type clusterPolls struct {
	cluster *model.AerospikeCluster
	// latest is the last successful poll, zero before the first. A published listing is never
	// changed, so a run may read it without the lock.
	latest JobListing
	// polled is closed by the next successful poll, which then replaces it.
	polled chan struct{}
	// waiters counts the runs waiting in Next.
	waiters int
}

var _ JobPoller = (*jobPoller)(nil)

// NewJobPoller returns a JobPoller of the clusters of routines.
func NewJobPoller(routines routineProvider, clientManager aerospike.ClientManager) JobPoller {
	return &jobPoller{
		routines:      routines,
		clientManager: clientManager,
		logger:        slog.Default(),
		clusters:      make(map[uint64]*clusterPolls),
	}
}

// Start polls at once and then every jobPollInterval until ctx is canceled.
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
		}
	}
}

// Next waits for the first successful poll of cluster that began after after.
func (p *jobPoller) Next(ctx context.Context, cluster *model.AerospikeCluster, after time.Time) (JobListing, error) {
	p.mu.Lock()
	c := p.clusterLocked(cluster)
	c.waiters++
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		c.waiters--
		p.mu.Unlock()
	}()

	for {
		p.mu.Lock()
		latest, polled := c.latest, c.polled
		p.mu.Unlock()

		if latest.At.After(after) {
			return latest, nil
		}

		select {
		case <-ctx.Done():
			return JobListing{}, ctx.Err()
		case <-polled:
		}
	}
}

// poll lists the jobs of each cluster to poll. A cluster whose listing fails keeps its last
// successful poll, and its waiters wait for the next one.
func (p *jobPoller) poll(ctx context.Context) {
	routines := p.routines.Routines()

	for _, c := range p.clustersToPoll(routines) {
		at := time.Now()

		jobs, err := p.listJobs(ctx, c.cluster)
		if err != nil {
			p.logger.Warn("Failed to list server backup jobs",
				slog.String("cluster", c.cluster.ClusterLabel), attr.Error(err))

			continue
		}

		p.publish(c, JobListing{Jobs: jobsByID(jobs), At: at}, jobs, routines)
	}
}

// clustersToPoll returns the clusters routines back up and those a run waits on, and forgets the
// others.
func (p *jobPoller) clustersToPoll(routines map[string]*model.BackupRoutine) []*clusterPolls {
	p.mu.Lock()
	defer p.mu.Unlock()

	used := make(map[*clusterPolls]bool)
	for _, routine := range routines {
		used[p.clusterLocked(routine.SourceCluster)] = true
	}

	clusters := make([]*clusterPolls, 0, len(p.clusters))
	for key, c := range p.clusters {
		if !used[c] && c.waiters == 0 {
			delete(p.clusters, key)
			continue
		}

		clusters = append(clusters, c)
	}

	return clusters
}

// clusterLocked returns what the poller knows of cluster, adding it if it is new. The caller must
// hold mu.
func (p *jobPoller) clusterLocked(cluster *model.AerospikeCluster) *clusterPolls {
	key := cluster.Hash()
	c, ok := p.clusters[key]
	if !ok {
		c = &clusterPolls{cluster: cluster, polled: make(chan struct{})}
		p.clusters[key] = c
	}

	return c
}

// publish logs what changed since the cluster's last successful poll, makes listing its latest
// and wakes its waiters.
func (p *jobPoller) publish(
	c *clusterPolls,
	listing JobListing,
	jobs []*infomodels.ResponseBackupState,
	routines map[string]*model.BackupRoutine,
) {
	p.report(c.cluster, jobs, routines, c.latest.Jobs)

	p.mu.Lock()
	defer p.mu.Unlock()

	c.latest = listing
	close(c.polled)
	c.polled = make(chan struct{})
}

func jobsByID(jobs []*infomodels.ResponseBackupState) map[string]*infomodels.ResponseBackupState {
	byID := make(map[string]*infomodels.ResponseBackupState, len(jobs))
	for _, job := range jobs {
		byID[job.JobID] = job
	}

	return byID
}

// listJobs lists the backup jobs of cluster, bounded by jobPollInterval.
func (p *jobPoller) listJobs(
	ctx context.Context,
	cluster *model.AerospikeCluster,
) ([]*infomodels.ResponseBackupState, error) {
	ctx, cancel := context.WithTimeout(ctx, jobPollInterval)
	defer cancel()

	client, err := p.clientManager.GetClient(ctx, cluster, nil, p.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	defer p.clientManager.Close(client)

	return client.InfoClient().ListBackupJobs(ctx)
}

// report logs each job of cluster that is new since the previous poll or changed its state. On a
// cluster's first poll, when previous is nil, the jobs that already ended are not logged: each
// node keeps up to 100 of them.
func (p *jobPoller) report(
	cluster *model.AerospikeCluster,
	jobs []*infomodels.ResponseBackupState,
	routines map[string]*model.BackupRoutine,
	previous map[string]*infomodels.ResponseBackupState,
) {
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
			attr.Routine(routineOf(job, cluster, routines)))
	}
}

// routineOf returns the name of the routine of cluster whose bucket and path job writes to, or ""
// if job writes outside every routine's path. The server ends the path it reports with "/".
func routineOf(
	job *infomodels.ResponseBackupState,
	cluster *model.AerospikeCluster,
	routines map[string]*model.BackupRoutine,
) string {
	for _, routine := range routines {
		storage, ok := routine.Storage.(*model.S3Storage)
		if ok && routine.SourceCluster.Hash() == cluster.Hash() && storage.Bucket == job.S3Bucket &&
			routinePath(storage, routine.Name)+"/" == job.Path {
			return routine.Name
		}
	}

	return ""
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
