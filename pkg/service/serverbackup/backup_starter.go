package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// backupStarter starts a routine's server backup: one job per namespace, all at once. It does not
// wait for the jobs: the job poller follows them and reports the backup when they end.
type backupStarter struct {
	startController   service.StartController
	reporter          service.BackupReporter
	runs              *Runs
	clientManager     aerospike.ClientManager
	namespaceResolver aerospike.NamespaceResolver
	catalog           Catalog
}

var _ service.BackupOrchestrator = (*backupStarter)(nil)

// NewBackupStarter returns the BackupOrchestrator of the server mode. It opens a run in runs for
// each backup it starts. catalog lists the server backups an incremental backup of a namespace
// builds on.
func NewBackupStarter(
	startController service.StartController,
	reporter service.BackupReporter,
	runs *Runs,
	clientManager aerospike.ClientManager,
	namespaceResolver aerospike.NamespaceResolver,
	catalog Catalog,
) service.BackupOrchestrator {
	return &backupStarter{
		startController:   startController,
		reporter:          reporter,
		runs:              runs,
		clientManager:     clientManager,
		namespaceResolver: namespaceResolver,
		catalog:           catalog,
	}
}

// Backup starts a backup of the routine if the start rules allow it, once its cluster has been
// listed: the rules count the backups that already run on it. A backup that starts no job is
// reported here.
func (s *backupStarter) Backup(
	ctx context.Context,
	routine *model.BackupRoutine,
	now time.Time,
	backupType model.BackupType,
) {
	logger := slog.With(attr.Routine(routine.Name))
	s.runs.awaitListing(ctx, routine.SourceCluster)

	release, err := s.startController.TryStart(routine, now, backupType)
	if err != nil {
		s.reporter.Report(routine.Name, backupType, 0, err, logger)
		return
	}
	defer release()

	service.LogBackupStarted(logger, routine, now, backupType)
	opened := s.runs.open(routine, backupType, now)
	jobs, startErr := s.startJobs(ctx, routine, backupType, now, logger)
	if failed := s.runs.close(opened, jobs, startErr); failed != nil {
		s.reporter.Report(routine.Name, backupType, time.Since(now),
			fmt.Errorf("%s backup failed: %w", backupType, failed), logger)
	}
}

// startJobs starts the job of each namespace of the routine, all at once. It returns the jobs that
// started, in the namespaces' order, and the errors of the others. A job whose start has no known
// outcome counts as started: the listings tell whether it runs.
func (s *backupStarter) startJobs(
	ctx context.Context,
	routine *model.BackupRoutine,
	backupType model.BackupType,
	now time.Time,
	logger *slog.Logger,
) ([]startedJob, error) {
	namespaces, err := s.namespaceResolver.ResolveNamespaces(ctx, routine, logger)
	if err != nil {
		return nil, err
	}

	bounds, err := s.lowerBounds(ctx, routine, backupType)
	if err != nil {
		return nil, err
	}

	client, err := s.clientManager.GetClient(ctx, routine.SourceCluster, nil, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup client: %w", err)
	}
	defer s.clientManager.Close(client)

	var wg sync.WaitGroup
	jobs := make([]startedJob, len(namespaces))
	errs := make([]error, len(namespaces))
	for i, namespace := range namespaces {
		wg.Go(func() {
			request, err := backupRequest(routine, backupType, now, lowerBound(bounds, namespace), namespace)
			if err != nil {
				errs[i] = fmt.Errorf("namespace %s: %w", namespace, err)
				return
			}

			jobID, err := client.InfoClient().StartBackup(ctx, request)
			switch {
			case err == nil:
				logger.Info("Server backup started", slog.String("namespace", namespace), slog.String("jobId", jobID))
			case errors.Is(err, asinfo.ErrOutcomeUnknown):
				logger.Warn("Server backup start has no known outcome: the listings tell whether it runs",
					slog.String("namespace", namespace), slog.String("jobId", jobID), attr.Error(err))
			default:
				errs[i] = fmt.Errorf("namespace %s: failed to start server backup: %w", namespace, err)
				return
			}

			jobs[i] = startedJob{namespace: namespace, id: jobID}
		})
	}
	wg.Wait()

	return slices.DeleteFunc(jobs, func(job startedJob) bool { return job.id == "" }), errors.Join(errs...)
}

// backupRequest returns the request that backs the namespace up for the routine: from modifiedAfter
// if set, and, for a sealed backup, up to now.
func backupRequest(
	routine *model.BackupRoutine,
	backupType model.BackupType,
	now time.Time,
	modifiedAfter *time.Time,
	namespace string,
) (*infomodels.RequestBackup, error) {
	common, err := withStorage(infomodels.RequestCommon{
		Namespace: namespace,
		SetList:   strings.Join(routine.SetList, ","),
		FilterExp: routine.FilterExpression,
		NoIndexes: routine.BackupPolicy.NoIndexes,
		NoUDFs:    routine.BackupPolicy.NoUdfs,
	}, routine.Storage, routine.Name)
	if err != nil {
		return nil, err
	}

	policy := routine.BackupPolicy
	// An unset limit is sent as zero. The info client leaves zero fields out of the command, so
	// the server uses its own default.
	// As in scan mode, only a full backup stores the cluster configuration, and only when asked.
	withClusterConfig := backupType == model.BackupTypeFull && policy.WithClusterConfigOrDefault()

	return &infomodels.RequestBackup{
		RequestCommon:       common,
		NoConfig:            new(!withClusterConfig),
		BinList:             strings.Join(routine.BinList, ","),
		ModifiedAfter:       unixSeconds(modifiedAfter),
		ModifiedBefore:      unixSeconds(policy.UpperBound(now)),
		BackupRPS:           ptr.ValueOrZero(policy.RecordsPerSecond),
		BackupWorkerThreads: ptr.ValueOrZero(policy.Parallel),
		BackupUploadThreads: ptr.ValueOrZero(policy.ParallelWrite),
	}, nil
}

// lowerBounds returns, per namespace, the time an incremental backup of it starts from: the time
// of the namespace's last full backup in the cumulative mode, otherwise of its last backup of
// either type. A namespace with no backup has no entry. A full backup has no lower bound, so the
// map is nil. The storage is listed once for all namespaces of the run.
func (s *backupStarter) lowerBounds(
	ctx context.Context,
	routine *model.BackupRoutine,
	backupType model.BackupType,
) (map[string]time.Time, error) {
	if backupType == model.BackupTypeFull {
		return nil, nil
	}

	backupTypes := []model.BackupType{model.BackupTypeFull}
	if routine.BackupPolicy.GetIncrModeOrDefault() != model.IncrModeCumulative {
		backupTypes = append(backupTypes, model.BackupTypeIncremental)
	}

	bounds, err := s.catalog.LastBackupTimes(ctx, routine, backupTypes...)
	if err != nil {
		return nil, fmt.Errorf("failed to read the last backups of routine %s: %w", routine.Name, err)
	}

	return bounds, nil
}

// lowerBound returns the namespace's bound in bounds, or none if it has no backup to build on.
func lowerBound(bounds map[string]time.Time, namespace string) *time.Time {
	bound, ok := bounds[namespace]
	if !ok {
		return nil
	}

	return &bound
}

func unixSeconds(t *time.Time) string {
	if t == nil {
		return ""
	}

	return strconv.FormatInt(t.Unix(), 10)
}

func newServerBackupJob(status *infomodels.ResponseBackupState) model.ServerBackupJob {
	return model.ServerBackupJob{
		JobID:             status.JobID,
		Namespace:         status.Namespace,
		State:             string(status.State),
		ErrorReason:       status.ErrorReason,
		AbortOrigin:       string(status.AbortOrigin),
		StartTime:         nonZeroTime(status.StartTime),
		FinishTime:        nonZeroTime(status.FinishTime),
		RecordsBackedUp:   status.RecsBackedUp,
		BytesBackedUp:     status.BytesBackedUp,
		RecordsReadFailed: status.RecsReadFailed,
	}
}

func nonZeroTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}

	return &t
}
