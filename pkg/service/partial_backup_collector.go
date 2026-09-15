package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// partialBackupMinAge is how old a folder without metadata has to be before the sweep removes
// it. The sweep runs once, at startup, so no run of this process can be older than the process
// itself: the age only keeps the sweep away from a backup this service starts while it is
// still listing, and from one written by another service that shares the storage.
const partialBackupMinAge = 24 * time.Hour

// PartialBackupCollector removes the folders of backup runs that never finished.
//
// A backup writes its data first and its metadata last, so a timestamp folder without a
// metadata file holds the remains of a run that did not complete. A run that fails or is
// canceled cleans up after itself; one killed with the process - SIGKILL, an OOM - cannot, and
// its folder is then invisible to retention, which only walks folders that have metadata. Left
// alone the data accumulates forever and stays restorable by path.
type PartialBackupCollector interface {
	// Start sweeps every configured routine once, in the background, and returns.
	Start(ctx context.Context)
}

type partialBackupCollector struct {
	routines   routineProvider
	catalog    BackupCatalog
	operations storage.Operations
}

var _ PartialBackupCollector = (*partialBackupCollector)(nil)

// NewPartialBackupCollector returns a PartialBackupCollector.
func NewPartialBackupCollector(
	routines routineProvider,
	catalog BackupCatalog,
	operations storage.Operations,
) PartialBackupCollector {
	return &partialBackupCollector{
		routines:   routines,
		catalog:    catalog,
		operations: operations,
	}
}

// Start sweeps in the background: the storage may be a cold cloud client, and nothing else
// waits on the result.
func (c *partialBackupCollector) Start(ctx context.Context) {
	go c.collect(ctx, time.Now())
}

func (c *partialBackupCollector) collect(ctx context.Context, now time.Time) {
	for _, routine := range c.routines.Routines() {
		if err := c.collectRoutine(ctx, routine, now); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}

			slog.Error("Failed to remove the folders of unfinished backups",
				attr.Routine(routine.Name), attr.Error(err))
		}
	}
}

func (c *partialBackupCollector) collectRoutine(
	ctx context.Context, routine *model.BackupRoutine, now time.Time,
) error {
	var errs error
	for _, backupType := range []model.BackupType{model.BackupTypeFull, model.BackupTypeIncremental} {
		folders, err := c.partialFolders(ctx, routine, backupType, now)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		for _, folder := range folders {
			if err := c.catalog.Delete(ctx, routine, folder); err != nil {
				errs = errors.Join(errs, fmt.Errorf("failed to delete folder at %v: %w", folder, err))
			}
		}
	}

	return errs
}

// partialFolders lists the timestamp folders of one backup type that hold no metadata file and
// are older than partialBackupMinAge.
func (c *partialBackupCollector) partialFolders(
	ctx context.Context, routine *model.BackupRoutine, backupType model.BackupType, now time.Time,
) ([]string, error) {
	root := backupRootPath(routine.Name, backupType)

	files, err := c.operations.ReadFileNames(ctx, routine.Storage, root, "", nil)
	if err != nil {
		return nil, fmt.Errorf("read file names in %s: %w", root, err)
	}
	files = pathsRelativeToStorage(files, path.Clean(routine.Storage.GetPath()))

	started := make(map[string]time.Time)
	finished := make(map[string]struct{})
	for _, file := range files {
		folder, timestamp, ok := timestampFolderOf(file, root)
		if !ok {
			continue
		}

		started[folder] = timestamp
		if path.Base(file) == metadataFile {
			finished[folder] = struct{}{}
		}
	}

	partial := make([]string, 0, len(started))
	for folder, timestamp := range started {
		if _, complete := finished[folder]; complete {
			continue
		}
		if now.Sub(timestamp) < partialBackupMinAge {
			continue
		}
		partial = append(partial, folder)
	}
	slices.Sort(partial) // a stable order, so the log of a sweep reads the same way twice

	return partial, nil
}

// timestampFolderOf returns the timestamp folder a file belongs to, and when that folder's run
// started. The layout is <root>/<unix millis>[_<formatted>]/..., so the folder name carries the
// start time; a path that does not fit the layout belongs to no run and is ignored.
func timestampFolderOf(file, root string) (folder string, timestamp time.Time, ok bool) {
	rest, found := strings.CutPrefix(path.Clean(file), root+"/")
	if !found {
		return "", time.Time{}, false
	}

	name, _, nested := strings.Cut(rest, "/")
	if !nested { // a file sitting directly in the root, not a run's folder
		return "", time.Time{}, false
	}

	millis, err := strconv.ParseInt(name[:min(len(name), timestampDigits)], 10, 64)
	if err != nil || len(name) < timestampDigits {
		return "", time.Time{}, false
	}

	return path.Join(root, name), time.UnixMilli(millis), true
}
