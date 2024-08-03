package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/aws/smithy-go/ptr"
)

// RestoreMemory implements the RestoreService interface.
// Stores job information locally within a map.
type RestoreMemory struct {
	config          *model.Config
	restoreJobs     *JobsHolder
	restoreService  shared.Restore
	backends        BackendsHolder
	asClientCreator ASClientCreator
}

var _ RestoreService = (*RestoreMemory)(nil)

// NewRestoreMemory returns a new RestoreMemory instance.
func NewRestoreMemory(backends BackendsHolder, config *model.Config, restoreService shared.Restore) *RestoreMemory {
	return &RestoreMemory{
		restoreJobs:     NewJobsHolder(),
		restoreService:  restoreService,
		backends:        backends,
		config:          config,
		asClientCreator: &AerospikeClientCreator{},
	}
}

func (r *RestoreMemory) Restore(request *model.RestoreRequestInternal) (RestoreJobID, error) {
	jobID := r.restoreJobs.newJob()
	if err := validateStorageContainsBackup(request.SourceStorage); err != nil {
		return 0, err
	}

	ctx := context.TODO()
	go func() {
		client, err := r.initClient(request.DestinationCuster, jobID)
		if err != nil {
			return
		}
		defer r.asClientCreator.Close(client)

		restoreResult, err := r.restoreService.RestoreRun(ctx, client, request)
		if err != nil {
			r.restoreJobs.setFailed(jobID, fmt.Errorf("failed restore operation: %w", err))
			return
		}
		r.restoreJobs.increaseStats(jobID, restoreResult)
		r.restoreJobs.setDone(jobID)
	}()

	return jobID, nil
}

func (r *RestoreMemory) initClient(cluster *model.AerospikeCluster, jobID RestoreJobID) (*aerospike.Client, error) {
	client, aerr := r.asClientCreator.NewClient(
		cluster.ASClientPolicy(),
		cluster.ASClientHosts()...)
	if aerr != nil {
		err := fmt.Errorf("failed to connect to aerospike cluster, %w", aerr)
		slog.Error("Failed to restore by timestamp", "cluster", cluster, "err", err)
		r.restoreJobs.setFailed(jobID, err)
		return nil, err
	}
	return client, nil
}

func (r *RestoreMemory) RestoreByTime(request *model.RestoreTimestampRequest) (RestoreJobID, error) {
	reader, found := r.backends.GetReader(request.Routine)
	if !found {
		return 0, fmt.Errorf("backend '%s' not found for restore", request.Routine)
	}
	fullBackups, err := reader.FindLastFullBackup(time.UnixMilli(request.Time))
	if err != nil {
		return 0, fmt.Errorf("last full backup not found: %v", err)
	}
	jobID := r.restoreJobs.newJob()
	ctx := context.TODO()
	go r.restoreByTimeSync(ctx, reader, request, jobID, fullBackups)

	return jobID, nil
}

func (r *RestoreMemory) restoreByTimeSync(
	ctx context.Context,
	backend BackupListReader,
	request *model.RestoreTimestampRequest,
	jobID RestoreJobID,
	fullBackups []model.BackupDetails,
) {
	client, err := r.initClient(request.DestinationCuster, jobID)
	if err != nil {
		return
	}
	defer r.asClientCreator.Close(client)

	var wg sync.WaitGroup

	for _, nsBackup := range fullBackups {
		wg.Add(1)
		go func(nsBackup model.BackupDetails) {
			defer wg.Done()
			if err := r.restoreNamespace(ctx, client, backend, request, jobID, nsBackup); err != nil {
				slog.Error("Failed to restore by timestamp", "routine", request.Routine, "err", err)
				r.restoreJobs.setFailed(jobID, err)
				return
			}
		}(nsBackup)
	}

	wg.Wait()

	r.restoreJobs.setDone(jobID)
}

func (r *RestoreMemory) restoreNamespace(
	ctx context.Context,
	client *aerospike.Client,
	backend BackupListReader,
	request *model.RestoreTimestampRequest,
	jobID RestoreJobID, fullBackup model.BackupDetails,
) error {
	result, err := r.restoreFromPath(ctx, client, request, fullBackup.Key)
	if err != nil {
		return fmt.Errorf("could not restore full backup for namespace %s: %v", fullBackup.Namespace, err)
	}
	r.restoreJobs.increaseStats(jobID, result)

	bounds, err := model.NewTimeBounds(&fullBackup.Created, ptr.Time(time.UnixMilli(request.Time)))
	if err != nil {
		return err
	}

	incrementalBackups, err := backend.FindIncrementalBackupsForNamespace(bounds, fullBackup.Namespace)
	if err != nil {
		return fmt.Errorf("could not find incremental backups for namespace %s: %v", fullBackup.Namespace, err)
	}
	slog.Info("Apply incremental backups", "size", len(incrementalBackups))
	for _, incrBackup := range incrementalBackups {
		result, err := r.restoreFromPath(ctx, client, request, incrBackup.Key)
		if err != nil {
			return fmt.Errorf("could not restore incremental backup %s: %v", *incrBackup.Key, err)
		}
		r.restoreJobs.increaseStats(jobID, result)
	}

	return nil
}

func (r *RestoreMemory) restoreFromPath(
	ctx context.Context,
	client *aerospike.Client,
	request *model.RestoreTimestampRequest,
	backupPath *string,
) (*model.RestoreResult, error) {
	restoreRequest := r.toRestoreRequest(request)
	restoreResult, err := r.restoreService.RestoreRun(ctx,
		client,
		&model.RestoreRequestInternal{
			RestoreRequest: *restoreRequest,
			Dir:            backupPath,
		})
	if err != nil {
		return nil, fmt.Errorf("could not restore backup at %s: %w", *backupPath, err)
	}

	return restoreResult, nil
}

func (r *RestoreMemory) toRestoreRequest(request *model.RestoreTimestampRequest) *model.RestoreRequest {
	routine := r.config.BackupRoutines[request.Routine]
	storage := r.config.Storage[routine.Storage]
	return model.NewRestoreRequest(
		request.DestinationCuster,
		request.Policy,
		storage,
		request.SecretAgent,
	)
}

// JobStatus returns the status of the job with the given id.
func (r *RestoreMemory) JobStatus(jobID RestoreJobID) (*model.RestoreJobStatus, error) {
	return r.restoreJobs.getStatus(jobID)
}

func validateStorageContainsBackup(storage *model.Storage) error {
	switch storage.Type {
	case model.Local:
		return validatePathContainsBackup(*storage.Path)
	case model.S3:
		s3context, err := NewS3Context(storage)
		if err != nil {
			return err
		}
		return s3context.validateStorageContainsBackup()
	}
	return nil
}

// ASClientCreator manages creation and close of aerospike connection.
// Required to be able to mock it in tests.
type ASClientCreator interface {
	NewClient(policy *aerospike.ClientPolicy, hosts ...*aerospike.Host) (*aerospike.Client, error)
	Close(client *aerospike.Client)
}

type AerospikeClientCreator struct{}

func (a *AerospikeClientCreator) Close(client *aerospike.Client) {
	client.Close()
}

func (a *AerospikeClientCreator) NewClient(policy *aerospike.ClientPolicy, hosts ...*aerospike.Host,
) (*aerospike.Client, error) {
	return aerospike.NewClientWithPolicyAndHost(policy, hosts...)
}
