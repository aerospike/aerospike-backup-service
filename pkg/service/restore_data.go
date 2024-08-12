package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/smithy-go/ptr"
	"github.com/prometheus/client_golang/prometheus"
)

var errBackendNotFound = errors.New("backend not found")
var errBackupNotFound = errors.New("backup not found")

// dataRestorer implements the RestoreManager interface.
// Stores job information locally within a map.
type dataRestorer struct {
	configRetriever
	config          *model.Config
	restoreJobs     *JobsHolder
	restoreService  Restore
	backends        BackendsHolder
	asClientCreator ASClientCreator
}

var _ RestoreManager = (*dataRestorer)(nil)

// NewRestoreManager returns a new dataRestorer instance.
func NewRestoreManager(backends BackendsHolder, config *model.Config,
	restoreService Restore) RestoreManager {
	return &dataRestorer{
		configRetriever: configRetriever{
			backends,
		},
		restoreJobs:     NewJobsHolder(),
		restoreService:  restoreService,
		backends:        backends,
		config:          config,
		asClientCreator: &AerospikeClientCreator{},
	}
}

func (r *dataRestorer) Restore(request *model.RestoreRequestInternal,
) (model.RestoreJobID, error) {
	jobID := r.restoreJobs.newJob()
	totalRecords, err := validateStorageContainsBackup(request.SourceStorage)
	if err != nil {
		return 0, err
	}

	ctx := context.TODO()
	go func() {
		client, err := r.initClient(request.DestinationCuster, jobID)
		if err != nil {
			return
		}
		defer r.asClientCreator.Close(client)

		handler, err := r.restoreService.RestoreRun(ctx, client, request)
		if err != nil {
			r.restoreJobs.setFailed(jobID, fmt.Errorf("failed to start restore operation: %w", err))
			return
		}
		r.restoreJobs.addTotalRecords(jobID, totalRecords)
		r.restoreJobs.addHandler(jobID, handler)

		// Wait for the restore operation to complete
		err = handler.Wait()
		if err != nil {
			r.restoreJobs.setFailed(jobID, fmt.Errorf("failed restore operation: %w", err))
			return
		}

		r.restoreJobs.setDone(jobID)
	}()

	return jobID, nil
}

func (r *dataRestorer) initClient(cluster *model.AerospikeCluster, jobID model.RestoreJobID,
) (*aerospike.Client, error) {
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

func (r *dataRestorer) RestoreByTime(request *model.RestoreTimestampRequest,
) (model.RestoreJobID, error) {
	reader, found := r.backends.GetReader(request.Routine)
	if !found {
		return 0, fmt.Errorf("%w: routine %s", errBackendNotFound, request.Routine)
	}
	timestamp := time.UnixMilli(request.Time)
	fullBackups, err := reader.FindLastFullBackup(timestamp)
	if err != nil {
		return 0, fmt.Errorf("restore failed: %w", err)
	}
	jobID := r.restoreJobs.newJob()
	ctx := context.TODO()
	go r.restoreByTimeSync(ctx, reader, request, jobID, fullBackups)

	return jobID, nil
}

func (r *dataRestorer) restoreByTimeSync(
	ctx context.Context,
	backend BackupListReader,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	fullBackups []model.BackupDetails,
) {
	client, err := r.initClient(request.DestinationCuster, jobID)
	if err != nil {
		return
	}
	defer r.asClientCreator.Close(client)

	var wg sync.WaitGroup

	multiError := prometheus.MultiError{}
	for _, nsBackup := range fullBackups {
		wg.Add(1)
		go func(nsBackup model.BackupDetails) {
			defer wg.Done()
			if err := r.restoreNamespace(ctx, client, backend, request, jobID, nsBackup); err != nil {
				multiError.Append(
					fmt.Errorf("failed to restore routine %s, namespace %s by timestamp: %w",
						request.Routine, nsBackup.Namespace, err))
			}
		}(nsBackup)
	}

	wg.Wait()

	err = multiError.MaybeUnwrap()
	if err != nil {
		r.restoreJobs.setFailed(jobID, err)
		return
	}

	r.restoreJobs.setDone(jobID)
}

func (r *dataRestorer) restoreNamespace(
	ctx context.Context,
	client *aerospike.Client,
	backend BackupListReader,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	fullBackup model.BackupDetails,
) error {
	allBackups := []model.BackupDetails{fullBackup}

	// Find incremental backups
	bounds, err := model.NewTimeBounds(&fullBackup.Created, ptr.Time(time.UnixMilli(request.Time)))
	if err != nil {
		return err
	}

	incrementalBackups, err := backend.FindIncrementalBackupsForNamespace(bounds,
		fullBackup.Namespace)
	if err != nil {
		return fmt.Errorf("could not find incremental backups for namespace %s: %w",
			fullBackup.Namespace, err)
	}

	// Append incremental backups to allBackups
	allBackups = append(allBackups, incrementalBackups...)

	for _, b := range allBackups {
		r.restoreJobs.addTotalRecords(jobID, b.RecordCount)
	}

	// Now restore all backups in order
	for _, b := range allBackups {
		handler, err := r.restoreFromPath(ctx, client, request, b.Key)
		if err != nil {
			return err
		}
		r.restoreJobs.addHandler(jobID, handler)

		err = handler.Wait()
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *dataRestorer) restoreFromPath(
	ctx context.Context,
	client *aerospike.Client,
	request *model.RestoreTimestampRequest,
	backupPath *string,
) (RestoreHandler, error) {
	restoreRequest := r.toRestoreRequest(request)
	handler, err := r.restoreService.RestoreRun(ctx,
		client,
		&model.RestoreRequestInternal{
			RestoreRequest: *restoreRequest,
			Dir:            backupPath,
		})
	if err != nil {
		return nil, fmt.Errorf("could not start restore from backup at %s: %w", *backupPath, err)
	}

	return handler, nil
}

func (r *dataRestorer) toRestoreRequest(request *model.RestoreTimestampRequest) *model.RestoreRequest {
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
func (r *dataRestorer) JobStatus(jobID model.RestoreJobID) (*model.RestoreJobStatus, error) {
	return r.restoreJobs.getStatus(jobID)
}

func validateStorageContainsBackup(storage *model.Storage) (uint64, error) {
	switch storage.Type {
	case model.Local:
		return validatePathContainsBackup(*storage.Path)
	case model.S3:
		s3context, err := NewS3Context(storage)
		if err != nil {
			return 0, err
		}
		return s3context.ValidateStorageContainsBackup()
	}
	return 0, nil
}

// ASClientCreator manages creation and close of aerospike connection.
// Required to be able to mock it in tests.
type ASClientCreator interface {
	NewClient(policy *aerospike.ClientPolicy, hosts ...*aerospike.Host) (*aerospike.Client, error)
	Close(client *aerospike.Client)
}

type AerospikeClientCreator struct{}

// Close closes the client.
func (a *AerospikeClientCreator) Close(client *aerospike.Client) {
	client.Close()
}

// NewClient returns a new [aerospike.Client].
func (a *AerospikeClientCreator) NewClient(policy *aerospike.ClientPolicy, hosts ...*aerospike.Host,
) (*aerospike.Client, error) {
	return aerospike.NewClientWithPolicyAndHost(policy, hosts...)
}
