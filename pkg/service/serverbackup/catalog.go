package serverbackup

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-client-go/v8/types"
	"github.com/aerospike/backup-go/pkg/asinfo"
	"github.com/aerospike/backup-go/pkg/server/lister"
	listermodels "github.com/aerospike/backup-go/pkg/server/lister/models"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// jobIDTimeLayout is the UTC timestamp a server job id starts with, e.g. 260316T142035-k3f9.
const jobIDTimeLayout = "060102T150405"

// S3ClientProvider returns the S3 client of a storage configuration.
type S3ClientProvider interface {
	// GetClient returns the S3 client for the given storage configuration.
	GetClient(ctx context.Context, st *model.S3Storage) (*awsS3.Client, error)
}

// Catalog reads and deletes server backups. It reads them from the metadata the server writes
// next to each of them, and their counts from the manifests, index and UDF files it writes with
// them. Like scan mode, it lists no empty incremental backup: there is nothing to restore from one.
type Catalog interface {
	service.BackupReader

	// GetBackup returns the backup with the key in the storage, without its counts, which restore
	// does not need. A key names one backup, because a server job backs up one namespace.
	GetBackup(ctx context.Context, st model.Storage, key string) (model.BackupDetails, error)

	// AllBackups returns every committed backup of the routine, oldest first, without its counts.
	// Unlike GetBackups, it returns the empty incremental backups too: they take storage as well.
	AllBackups(ctx context.Context, routine *model.BackupRoutine) ([]model.BackupDetails, error)

	// Delete deletes the folder of the backup with the key, with everything in it.
	Delete(ctx context.Context, st model.Storage, key string) error

	// LastBackupTimes returns, per namespace, the creation time of the routine's newest listed
	// backup of any of backupTypes. A namespace with no such backup has no entry.
	LastBackupTimes(
		ctx context.Context, routine *model.BackupRoutine, backupTypes ...model.BackupType,
	) (map[string]time.Time, error)
}

// backupSource reads a routine's server backups. *lister.Lister is one.
type backupSource interface {
	FetchAllMetadata(ctx context.Context) ([]listermodels.Metadata, error)
	GetMetadata(ctx context.Context, backupID string) (listermodels.Metadata, error)
	GetStatistics(ctx context.Context, backupID string) (listermodels.Statistics, error)
}

// sourceOpener opens the source of a routine's backups in storage.
type sourceOpener interface {
	open(ctx context.Context, st model.Storage, routineName string) (backupSource, error)
}

// catalog takes a lock per routine, so that no read of a routine's backups overlaps the deletion
// of one of them.
type catalog struct {
	locks      collections.LockMap // lock per routine
	sources    sourceOpener
	operations storage.Operations
}

var _ Catalog = (*catalog)(nil)

// NewCatalog returns a Catalog over the server backups in S3 storage. operations deletes them.
func NewCatalog(s3Clients S3ClientProvider, operations storage.Operations) Catalog {
	return &catalog{sources: newListerOpener(s3Clients), operations: operations}
}

// GetBackups returns the routine's listed server backups that match the filter, oldest first.
func (c *catalog) GetBackups(ctx context.Context, filter service.BackupFilter) ([]model.BackupDetails, error) {
	routineFilter, ok := filter.(*service.RoutineFilter)
	if !ok {
		return nil, fmt.Errorf("unsupported backup filter type %T", filter)
	}

	lock := c.locks.Get(routineFilter.RoutineName())
	lock.RLock()
	defer lock.RUnlock()

	return c.getRoutineBackups(ctx, routineFilter)
}

// getRoutineBackups reads the counts of only the backups the filter selects: they take a read of
// every manifest of the backup.
func (c *catalog) getRoutineBackups(ctx context.Context, filter *service.RoutineFilter) ([]model.BackupDetails, error) {
	source, backups, err := c.listBackups(ctx, filter.Storage(), filter.RoutineName())
	if err != nil {
		return nil, err
	}

	bounds := filter.TimeBounds()
	backups = slices.DeleteFunc(backups, func(b model.BackupDetails) bool {
		return b.BackupType() != filter.BackupType() || !bounds.Contains(b.Created)
	})

	if filter.OnlyLast() {
		for _, backup := range slices.Backward(backups) {
			listed, err := withStatistics(ctx, source, &backup)
			if err != nil {
				return nil, err
			}

			if listed {
				return []model.BackupDetails{backup}, nil
			}
		}

		return nil, nil
	}

	matching := make([]model.BackupDetails, 0, len(backups))
	for _, backup := range backups {
		listed, err := withStatistics(ctx, source, &backup)
		if err != nil {
			return nil, err
		}

		if listed {
			matching = append(matching, backup)
		}
	}

	return matching, nil
}

func (c *catalog) GetBackup(ctx context.Context, st model.Storage, key string) (model.BackupDetails, error) {
	routineName, jobID, err := parseBackupKey(key)
	if err != nil {
		return model.BackupDetails{}, err
	}

	lock := c.locks.Get(routineName)
	lock.RLock()
	defer lock.RUnlock()

	source, err := c.sources.open(ctx, st, routineName)
	if err != nil {
		return model.BackupDetails{}, err
	}

	metadata, err := source.GetMetadata(ctx, jobID)
	if err != nil {
		return model.BackupDetails{}, fmt.Errorf("failed to read server backup metadata: %w", err)
	}

	return toBackupDetails(metadata, st, routineName), nil
}

func (c *catalog) AllBackups(ctx context.Context, routine *model.BackupRoutine) ([]model.BackupDetails, error) {
	lock := c.locks.Get(routine.Name)
	lock.RLock()
	defer lock.RUnlock()

	_, backups, err := c.listBackups(ctx, routine.Storage, routine.Name)

	return backups, err
}

// Delete removes the backup's folder, whose path is the key. It refuses a key whose job id is
// malformed, because a listed key takes its id from the content of the backup's metadata file.
// Anything but a job id there, such as an empty id, would name a folder that holds other backups.
func (c *catalog) Delete(ctx context.Context, st model.Storage, key string) error {
	routineName, jobID, err := parseBackupKey(key)
	if err != nil {
		return err
	}

	if !asinfo.IsJobID(jobID) {
		return fmt.Errorf("%q is not a job id", jobID)
	}

	lock := c.locks.Get(routineName)
	lock.Lock()
	defer lock.Unlock()

	if err := c.operations.DeleteFolder(ctx, st, key); err != nil {
		return err
	}

	slog.Info("Deleted server backup", slog.String("key", key), attr.Routine(routineName))

	return nil
}

// LastBackupTimes reads the counts of only the incremental backups newer than their namespace's
// last listed backup, to tell whether they are empty.
func (c *catalog) LastBackupTimes(
	ctx context.Context,
	routine *model.BackupRoutine,
	backupTypes ...model.BackupType,
) (map[string]time.Time, error) {
	lock := c.locks.Get(routine.Name)
	lock.RLock()
	defer lock.RUnlock()

	source, backups, err := c.listBackups(ctx, routine.Storage, routine.Name)
	if err != nil {
		return nil, err
	}

	// Newest first, so that the first listed backup of a namespace is its last.
	slices.SortStableFunc(backups, func(a, b model.BackupDetails) int { return b.Created.Compare(a.Created) })

	last := make(map[string]time.Time)
	for _, backup := range backups {
		if _, found := last[backup.Namespace]; found || !slices.Contains(backupTypes, backup.BackupType()) {
			continue
		}

		if backup.BackupType() == model.BackupTypeIncremental {
			listed, err := withStatistics(ctx, source, &backup)
			if err != nil {
				return nil, err
			}

			if !listed {
				continue
			}
		}

		last[backup.Namespace] = backup.Created
	}

	return last, nil
}

// listBackups returns the routine's committed server backups in storage, oldest first, without
// their counts, and the source to read the counts from.
func (c *catalog) listBackups(
	ctx context.Context,
	st model.Storage,
	routineName string,
) (backupSource, []model.BackupDetails, error) {
	source, err := c.sources.open(ctx, st, routineName)
	if err != nil {
		return nil, nil, err
	}

	all, err := source.FetchAllMetadata(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list server backup metadata: %w", err)
	}

	backups := make([]model.BackupDetails, 0, len(all))
	for _, metadata := range all {
		if metadata.Status == listermodels.BackupStatusComplete {
			backups = append(backups, toBackupDetails(metadata, st, routineName))
		}
	}

	return source, backups, nil
}

// listerOpener opens a lister of a routine's server backups in S3 storage.
type listerOpener struct {
	s3Clients S3ClientProvider
}

func newListerOpener(s3Clients S3ClientProvider) sourceOpener {
	return listerOpener{s3Clients: s3Clients}
}

func (o listerOpener) open(ctx context.Context, st model.Storage, routineName string) (backupSource, error) {
	s3Storage, err := toS3Storage(st)
	if err != nil {
		return nil, err
	}

	client, err := o.s3Clients.GetClient(ctx, s3Storage)
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 client: %w", err)
	}

	return lister.NewLister(
		client, s3Storage.Bucket, routinePath(s3Storage, routineName), lister.WithLogger(slog.Default()),
	), nil
}

// toBackupDetails converts server metadata to the details of a backup of the routine, keyed by
// its routine and job id, without its counts. Its lower time bound, From, is what makes it
// incremental.
func toBackupDetails(metadata listermodels.Metadata, st model.Storage, routineName string) model.BackupDetails {
	var from time.Time
	if metadata.ModifiedAfterUnixS != 0 {
		from = time.Unix(metadata.ModifiedAfterUnixS, 0).UTC()
	}

	return model.NewBackupDetails(
		model.BackupMetadata{
			Created:     createdTime(metadata),
			Finished:    finishedTime(metadata),
			From:        from,
			Namespace:   metadata.Namespace,
			Compression: model.CompressionModeNone,
			Encryption:  model.EncryptionModeNone,
		},
		backupKey(routineName, metadata.BackupID),
		st,
	)
}

// withStatistics sets the counts of the backup from source. It reports whether the backup is
// listed: an empty incremental backup is not.
func withStatistics(ctx context.Context, source backupSource, backup *model.BackupDetails) (bool, error) {
	_, jobID, err := parseBackupKey(backup.Key)
	if err != nil {
		return false, err
	}

	stats, err := source.GetStatistics(ctx, jobID)
	if err != nil {
		return false, fmt.Errorf("failed to read the counts of server backup %s: %w", jobID, err)
	}

	backup.RecordCount = stats.RecordCount
	backup.ByteCount = stats.ByteCount
	backup.FileCount = stats.FileCount
	backup.SecondaryIndexCount = stats.IndexCount
	backup.UDFCount = stats.UDFCount

	return backup.BackupType() == model.BackupTypeFull || !isEmpty(stats), nil
}

// isEmpty reports whether a server backup holds no records: it has no segment files. A zero
// record count alone does not tell, since it counts only the base round.
func isEmpty(stats listermodels.Statistics) bool {
	return stats.FileCount == 0
}

// finishedTime returns the time the backup's data is consistent to: the checkpoint of a strict
// backup or the end of the base scan of a fuzzy one, whichever is set, in Citrusleaf epoch ms.
// A backup from a server build that records neither has no finish time.
func finishedTime(metadata listermodels.Metadata) time.Time {
	ms := metadata.CheckpointMs
	if ms == 0 {
		ms = metadata.BaseScanEndMs
	}

	if ms == 0 {
		return time.Time{}
	}

	return time.UnixMilli(ms + types.CITRUSLEAF_EPOCH*1000).UTC()
}

// createdTime returns the time the backup's data is as of. A sealed backup holds the records
// modified before its upper bound, which is the run's start, shared by every namespace of the
// run; the next incremental starts there, so that no record falls between the two. An unsealed
// backup has no upper bound and holds what its job saw, so it is as of the time in its job id.
func createdTime(metadata listermodels.Metadata) time.Time {
	if metadata.ModifiedBeforeUnixS != 0 {
		return time.Unix(metadata.ModifiedBeforeUnixS, 0).UTC()
	}

	return jobIDTime(metadata.BackupID)
}

// jobIDTime returns the UTC time a server job id was generated at, or zero for a malformed id.
func jobIDTime(jobID string) time.Time {
	timestamp, _, _ := strings.Cut(jobID, "-")

	t, err := time.Parse(jobIDTimeLayout, timestamp)
	if err != nil {
		return time.Time{}
	}

	return t
}
