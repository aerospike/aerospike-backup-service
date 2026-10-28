package serverbackup

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/backup-go/pkg/server/lister"
	listermodels "github.com/aerospike/backup-go/pkg/server/lister/models"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// jobIDTimeLayout is the UTC timestamp a server job id starts with, e.g. 260316T142035-k3f9.
const jobIDTimeLayout = "060102T150405"

// S3ClientProvider returns the S3 client of a storage configuration.
type S3ClientProvider interface {
	// GetClient returns the S3 client for the given storage configuration.
	GetClient(ctx context.Context, storage *model.S3Storage) (*awsS3.Client, error)
}

// catalog lists server backups from the metadata the server writes next to each of them.
type catalog struct {
	s3Clients S3ClientProvider
}

var _ service.BackupReader = (*catalog)(nil)

// NewCatalog returns a BackupReader over the server backups in S3 storage.
func NewCatalog(s3Clients S3ClientProvider) service.BackupReader {
	return &catalog{s3Clients: s3Clients}
}

// GetBackups returns the completed server backups that match the filter, oldest first.
func (c *catalog) GetBackups(ctx context.Context, filter service.BackupFilter) ([]model.BackupDetails, error) {
	switch f := filter.(type) {
	case *service.RoutineFilter:
		return c.getRoutineBackups(ctx, f)
	case *service.PathFilter:
		return c.getPathBackup(ctx, f)
	default:
		return nil, fmt.Errorf("unsupported backup filter type %T", filter)
	}
}

func (c *catalog) getRoutineBackups(ctx context.Context, filter *service.RoutineFilter) ([]model.BackupDetails, error) {
	backups, err := c.listBackups(ctx, filter.Storage())
	if err != nil {
		return nil, err
	}

	bounds := filter.TimeBounds()
	matching := make([]model.BackupDetails, 0, len(backups))
	for _, backup := range backups {
		if backup.BackupType() == filter.BackupType() && bounds.Contains(backup.Created) {
			matching = append(matching, backup)
		}
	}

	if filter.OnlyLast() && len(matching) > 0 {
		return matching[len(matching)-1:], nil
	}

	return matching, nil
}

// getPathBackup reads the backup whose job id is the filter's path.
func (c *catalog) getPathBackup(ctx context.Context, filter *service.PathFilter) ([]model.BackupDetails, error) {
	backupLister, err := c.newLister(ctx, filter.Storage())
	if err != nil {
		return nil, err
	}

	metadata, err := backupLister.GetMetadata(ctx, filter.Path())
	if err != nil {
		return nil, fmt.Errorf("failed to read server backup metadata: %w", err)
	}

	return []model.BackupDetails{toBackupDetails(metadata, filter.Storage())}, nil
}

// listBackups returns the completed server backups of the storage, oldest first.
func (c *catalog) listBackups(ctx context.Context, storage model.Storage) ([]model.BackupDetails, error) {
	backupLister, err := c.newLister(ctx, storage)
	if err != nil {
		return nil, err
	}

	all, err := backupLister.FetchAllMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list server backup metadata: %w", err)
	}

	backups := make([]model.BackupDetails, 0, len(all))
	for _, metadata := range all {
		if metadata.Status == listermodels.BackupStatusComplete {
			backups = append(backups, toBackupDetails(metadata, storage))
		}
	}

	return backups, nil
}

func (c *catalog) newLister(ctx context.Context, storage model.Storage) (*lister.Lister, error) {
	s3Storage, err := toS3Storage(storage)
	if err != nil {
		return nil, err
	}

	client, err := c.s3Clients.GetClient(ctx, s3Storage)
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 client: %w", err)
	}

	return lister.NewLister(client, s3Storage.Bucket, s3Storage.Path, lister.WithLogger(slog.Default())), nil
}

// toBackupDetails converts server metadata to the details of a backup keyed by its job id. It
// was created when its job started, at the time in the job id. Its lower time bound, From, is
// what makes it incremental. The metadata records no finish time and no record, byte, file,
// secondary index or UDF counts, so they are left at zero, except the file count: it is one,
// so that restore does not take the backup for an empty one.
func toBackupDetails(metadata listermodels.Metadata, storage model.Storage) model.BackupDetails {
	var from time.Time
	if metadata.ModifiedAfterUnixS != 0 {
		from = time.Unix(metadata.ModifiedAfterUnixS, 0).UTC()
	}

	return model.NewBackupDetails(
		model.BackupMetadata{
			Created:     jobIDTime(metadata.BackupID),
			From:        from,
			Namespace:   metadata.Namespace,
			FileCount:   1,
			Compression: model.CompressionModeNone,
			Encryption:  model.EncryptionModeNone,
		},
		metadata.BackupID,
		storage,
	)
}

// newJobID returns a job id for a job the service starts, in the shape the server's own
// backup job ids have: the UTC time and a random base36 salt.
func newJobID(now time.Time) string {
	const saltSpace = 36 * 36 * 36 * 36

	salt := strconv.FormatInt(rand.Int64N(saltSpace), 36) //nolint:gosec // uniqueness, not security
	return fmt.Sprintf("%s-%04s", now.UTC().Format(jobIDTimeLayout), salt)
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
