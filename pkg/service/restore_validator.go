package service

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	coll "github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/backup-go"
)

var ErrRestorePrerequisitesFailed = errors.New("restore pre-requisites failed")

// RestoreValidator checks whether a restore request can run: the backup encryption must match
// the request policy, a configured namespace remap must be scoped to exactly its source
// namespace, the destination cluster must have the target namespaces, and no routine may be
// backing up the same cluster and namespaces at that moment.
type RestoreValidator interface {
	// ValidatePath validates a restore from an explicit backup path. On top of the common checks
	// it requires the backups to hold data and to be taken at the same time.
	// A path without backup metadata is allowed and restored as is.
	ValidatePath(
		ctx context.Context,
		request *model.RestoreRequest,
		infoGetter backup.InfoGetter,
		backups []model.BackupDetails,
	) error
	// ValidateTimestamp validates point-in-time restore preconditions.
	ValidateTimestamp(
		ctx context.Context,
		request *model.RestoreTimestampRequest,
		infoGetter backup.InfoGetter,
		backupsByNamespace map[string][]model.BackupDetails,
	) error
}

type restoreValidator struct {
	startController StartController
	routines        routineProvider
}

var _ RestoreValidator = (*restoreValidator)(nil)

// NewRestoreValidator returns a RestoreValidator.
func NewRestoreValidator(
	startController StartController,
	routines routineProvider,
) RestoreValidator {
	return &restoreValidator{
		startController: startController,
		routines:        routines,
	}
}

// ValidatePath validates path-restore preconditions.
func (r *restoreValidator) ValidatePath(
	ctx context.Context,
	request *model.RestoreRequest,
	infoGetter backup.InfoGetter,
	backups []model.BackupDetails,
) error {
	if len(backups) == 0 {
		return nil // no backup metadata found; nothing to validate, will try to restore as-is.
	}

	if allBackupsEmpty(backups) {
		return fmt.Errorf("%w: backup metadata indicates there is no data to restore (file count is zero)",
			ErrRestorePrerequisitesFailed)
	}

	if err := validateBackupsCreatedAtTheSameTime(backups); err != nil {
		return fmt.Errorf("%w: %w", ErrRestorePrerequisitesFailed, err)
	}

	if err := validateBackupsEncryption(backups, request.Policy.EncryptionPolicy); err != nil {
		return fmt.Errorf("%w: %w", ErrRestorePrerequisitesFailed, err)
	}

	sourceNamespaces := sourceNamespacesFromBackups(backups)
	if err := r.validateNamespaceScope(ctx, request.DestinationCluster, request.Policy.Namespace,
		sourceNamespaces, infoGetter); err != nil {
		return fmt.Errorf("%w: %w", ErrRestorePrerequisitesFailed, err)
	}

	return nil
}

// ValidateTimestamp validates point-in-time restore preconditions.
func (r *restoreValidator) ValidateTimestamp(
	ctx context.Context,
	request *model.RestoreTimestampRequest,
	infoGetter backup.InfoGetter,
	backupsByNamespace map[string][]model.BackupDetails,
) error {
	backups := coll.Flatten(backupsByNamespace)
	if err := validateBackupsEncryption(backups, request.Policy.EncryptionPolicy); err != nil {
		return fmt.Errorf("%w: %w", ErrRestorePrerequisitesFailed, err)
	}

	sourceNamespaces := coll.Unique(slices.Collect(maps.Keys(backupsByNamespace)))
	if err := r.validateNamespaceScope(ctx, request.DestinationCluster, request.Policy.Namespace,
		sourceNamespaces, infoGetter); err != nil {
		return fmt.Errorf("%w: %w", ErrRestorePrerequisitesFailed, err)
	}

	return nil
}

// validateNamespaceScope validates a restore's namespace scope: a configured remap must apply
// to exactly its source namespace, the resulting destination namespaces must exist on the
// destination cluster, and none may conflict with a currently running backup.
//
// It gathers the facts these checks need up front — the destination cluster's real namespace
// list, and the scope of every backup currently running — and applies pure rules over them.
// Gathering facts is the only step here that touches the destination cluster or the routine
// registry; every other function called from here is a pure decision over the facts it's given.
func (r *restoreValidator) validateNamespaceScope(
	ctx context.Context,
	cluster model.AerospikeCluster,
	remapping *model.RestoreNamespace,
	sourceNamespaces []string,
	infoGetter backup.InfoGetter,
) error {
	if err := validateNamespaceRemapScope(remapping, sourceNamespaces); err != nil {
		return err
	}

	destinationNamespaces := destinationNamespacesForRestore(remapping, sourceNamespaces)

	clusterNamespaces, err := fetchClusterNamespaces(ctx, infoGetter)
	if err != nil {
		return err
	}
	if err := validateNamespacesExist(destinationNamespaces, clusterNamespaces); err != nil {
		return err
	}

	return validateNoRunningBackupConflict(cluster, destinationNamespaces, r.runningRoutines())
}

// runningRoutines fetches every backup routine that currently has a backup in flight, so a
// restore's namespace conflicts can be decided without going back to the registry per routine.
func (r *restoreValidator) runningRoutines() []*model.BackupRoutine {
	routines := r.routines.Routines()
	running := make([]*model.BackupRoutine, 0, len(routines))
	for _, routine := range routines {
		if r.startController.HasBackupRunning(routine) {
			running = append(running, routine)
		}
	}

	return running
}

// validateNoRunningBackupConflict reports an error when a running backup already occupies the
// cluster the restore writes to and one of its destination namespaces.
func validateNoRunningBackupConflict(
	cluster model.AerospikeCluster,
	destinationNamespaces []string,
	running []*model.BackupRoutine,
) error {
	clusterHash := cluster.Hash()
	for _, routine := range running {
		// Only block if the running backup reads from the cluster the restore writes to.
		if routine.SourceCluster.Hash() != clusterHash {
			continue
		}

		// A routine backing up the whole cluster conflicts with any restore into that cluster.
		if routine.BacksUpWholeCluster() {
			return errRestoreConflictsWithBackup(routine.Name, cluster, "all namespaces")
		}

		if ns, ok := coll.FirstCommon(destinationNamespaces, routine.Namespaces); ok {
			return errRestoreConflictsWithBackup(routine.Name, cluster, fmt.Sprintf("namespace %q", ns))
		}
	}

	return nil
}

// errRestoreConflictsWithBackup describes a restore blocked by a backup running on the same
// cluster; scope names what they collide on.
func errRestoreConflictsWithBackup(routineName string, cluster model.AerospikeCluster, scope string) error {
	return fmt.Errorf("restore not allowed during backups on routine %s (cluster %s, %s). "+
		"Please cancel existing backups jobs to perform restore", routineName, cluster.Label(), scope)
}

// fetchClusterNamespaces queries and returns the namespaces that currently exist on the
// destination cluster. It wraps the InfoGetter call with context-specific error messaging.
func fetchClusterNamespaces(ctx context.Context, infoGetter backup.InfoGetter) ([]string, error) {
	namespaces, err := infoGetter.GetNamespacesList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespaces from destination cluster: %w", err)
	}

	return namespaces, nil
}

// validateNamespacesExist reports an error naming the first destination namespace that does not
// exist on the destination cluster. destinationNamespaces is guaranteed non-empty: every restore
// resolves at least one source namespace, and every source namespace maps to a destination
// namespace before this runs (see destinationNamespacesForRestore).
func validateNamespacesExist(destinationNamespaces, clusterNamespaces []string) error {
	for _, ns := range destinationNamespaces {
		if !slices.Contains(clusterNamespaces, ns) {
			return fmt.Errorf("destination cluster does not have required namespace: %s", ns)
		}
	}

	return nil
}

// validateBackupsCreatedAtTheSameTime ensures all selected backups belong to the same snapshot.
func validateBackupsCreatedAtTheSameTime(backups []model.BackupDetails) error {
	for i, b := range backups {
		if !b.Created.Equal(backups[0].Created) {
			return fmt.Errorf("backup at index %d created at %s differs from first backup created at %s",
				i, b.Created.String(), backups[0].Created.String())
		}
	}

	return nil
}

// validateBackupsEncryption validates that the backups encryption matches the provided policy.
func validateBackupsEncryption(backups []model.BackupDetails, policy *model.EncryptionPolicy) error {
	for _, b := range backups {
		if err := policy.ValidateCanDecrypt(b.Encryption); err != nil {
			return err
		}
	}

	return nil
}

// validateNamespaceRemapScope ensures a configured namespace remap only applies to a restore
// scoped to exactly its source namespace. A remap is a strict 1:1 rewrite at record level:
// backup-go's changeNamespace processor rejects any record whose namespace differs from
// Source, so a restore that also touches other namespaces would fail record-by-record deep in
// the write pipeline instead of being rejected here with a clear reason.
func validateNamespaceRemapScope(remapping *model.RestoreNamespace, sourceNamespaces []string) error {
	if remapping == nil {
		return nil
	}

	if len(sourceNamespaces) == 0 || !allNamespacesEqual(sourceNamespaces, remapping.Source) {
		return fmt.Errorf(
			"namespace remap from %q requires the restore to be scoped to exactly that namespace, got %v",
			remapping.Source, sourceNamespaces)
	}

	return nil
}

// destinationNamespacesForRestore resolves the destination namespaces a restore writes to:
// each source namespace mapped through the policy's remapping, if any. A nil remapping is safe;
// RestoreNamespace.DestinationOr is nil-receiver safe and returns the source namespace unchanged.
func destinationNamespacesForRestore(remapping *model.RestoreNamespace, sourceNamespaces []string) []string {
	destinations := make([]string, len(sourceNamespaces))
	for i, ns := range sourceNamespaces {
		destinations[i] = remapping.DestinationOr(ns)
	}

	return destinations
}

// sourceNamespacesFromBackups extracts unique source namespaces from backup metadata entries.
func sourceNamespacesFromBackups(backups []model.BackupDetails) []string {
	namespaces := make([]string, 0, len(backups))
	for _, b := range backups {
		namespaces = append(namespaces, b.Namespace)
	}

	return coll.Unique(namespaces)
}

// allNamespacesEqual reports whether every namespace in the list equals target.
func allNamespacesEqual(namespaces []string, target string) bool {
	for _, ns := range namespaces {
		if ns != target {
			return false
		}
	}

	return true
}

func allBackupsEmpty(backups []model.BackupDetails) bool {
	for _, b := range backups {
		if b.FileCount > 0 {
			return false
		}
	}

	return true
}
