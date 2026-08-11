package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/backup-go"
)

var ErrRestorePrerequisitesFailed = errors.New("restore pre-requisites failed")

// RestoreValidator validates restore preconditions before actual execution starts.
type RestoreValidator interface {
	// ValidatePath validates path-restore preconditions.
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

type restoreValidatorImpl struct {
	startController StartController
	routines        routineProvider
}

// NewRestoreValidator creates a validator for restore operations.
func NewRestoreValidator(
	startController StartController,
	routines routineProvider,
) RestoreValidator {
	return &restoreValidatorImpl{
		startController: startController,
		routines:        routines,
	}
}

// ValidatePath validates path-restore preconditions.
func (r *restoreValidatorImpl) ValidatePath(
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
	destinationNamespaces := destinationNamespacesForRestore(request.Policy.Namespace, sourceNamespaces)

	if err := validateDestinationNamespaces(ctx, destinationNamespaces, infoGetter); err != nil {
		return fmt.Errorf("%w: %w", ErrRestorePrerequisitesFailed, err)
	}

	if err := r.checkRunningBackupsConflict(request.DestinationCluster, destinationNamespaces); err != nil {
		return fmt.Errorf("%w: %w", ErrRestorePrerequisitesFailed, err)
	}

	return nil
}

// ValidateTimestamp validates point-in-time restore preconditions.
func (r *restoreValidatorImpl) ValidateTimestamp(
	ctx context.Context,
	request *model.RestoreTimestampRequest,
	infoGetter backup.InfoGetter,
	backupsByNamespace map[string][]model.BackupDetails,
) error {
	backups := collections.Flatten(backupsByNamespace)
	if err := validateBackupsEncryption(backups, request.Policy.EncryptionPolicy); err != nil {
		return fmt.Errorf("%w: %w", ErrRestorePrerequisitesFailed, err)
	}

	sourceNamespaces := collections.Keys(backupsByNamespace)
	destinationNamespaces := destinationNamespacesForRestore(request.Policy.Namespace, sourceNamespaces)
	if err := validateDestinationNamespaces(ctx, destinationNamespaces, infoGetter); err != nil {
		return fmt.Errorf("%w: %w", ErrRestorePrerequisitesFailed, err)
	}

	if err := r.checkRunningBackupsConflict(request.DestinationCluster, destinationNamespaces); err != nil {
		return fmt.Errorf("%w: %w", ErrRestorePrerequisitesFailed, err)
	}

	return nil
}

// checkRunningBackupsConflict validates the provided destination cluster and namespaces
// against all currently active backup routines to prevent concurrent operations on the same data.
func (r *restoreValidatorImpl) checkRunningBackupsConflict(
	cluster model.AerospikeCluster,
	destinationNamespaces []string,
) error {
	clusterHash := cluster.Hash()
	for _, routine := range r.routines.Routines() {
		if !r.startController.HasBackupRunning(routine) {
			continue
		}

		// Only block if the running backup targets the same destination cluster as the restore.
		if routine.SourceCluster.Hash() != clusterHash {
			continue
		}

		// Block if there is any overlap between the destination namespaces of the restore
		// and the source namespaces of the running backup.
		if overlappingNS := namespacesOverlap(destinationNamespaces, routine.Namespaces); overlappingNS != "" {
			return fmt.Errorf("restore not allowed during backups on routine %s (cluster %s, namespace %q). "+
				"Please cancel existing backups jobs to perform restore", routine.Name, cluster.ToString(), overlappingNS)
		}
	}

	return nil
}

// validateDestinationNamespaces checks destination namespaces existence in destination cluster.
func validateDestinationNamespaces(
	ctx context.Context,
	destinationNamespaces []string,
	infoGetter backup.InfoGetter,
) error {
	// Empty list means "all namespaces"; no specific namespace existence check is required.
	if len(destinationNamespaces) == 0 {
		return nil
	}

	namespaces, err := infoGetter.GetNamespacesList(ctx)
	if err != nil {
		return fmt.Errorf("failed to get namespaces from destination cluster: %w", err)
	}

	for _, destNS := range destinationNamespaces {
		if !slices.Contains(namespaces, destNS) {
			return fmt.Errorf("destination cluster does not have required namespace: %s", destNS)
		}
	}

	return nil
}

// validateBackupsCreatedAtTheSameTime ensures all selected backups belong to the same snapshot.
func validateBackupsCreatedAtTheSameTime(backups []model.BackupDetails) error {
	for _, b := range backups {
		if !b.Created.Equal(backups[0].Created) {
			return fmt.Errorf("backups from different times were found: %s and %s",
				b.Created.String(), backups[0].Created.String())
		}
	}

	return nil
}

// validateBackupsEncryption validates that the backups encryption matches the provided policy.
func validateBackupsEncryption(backups []model.BackupDetails, policy *model.EncryptionPolicy) error {
	for _, b := range backups {
		if b.Encryption == "" || b.Encryption == model.EncryptNone {
			continue
		}
		if policy == nil {
			return fmt.Errorf("backup is encrypted with mode '%s', "+
				"but no encryption policy was provided in the restore request", b.Encryption)
		}

		if policy.Mode != b.Encryption {
			return fmt.Errorf("backup is encrypted with mode '%s', "+
				"but the provided encryption policy specifies mode '%s'", b.Encryption, policy.Mode)
		}
		if policy.KeyFile == "" &&
			policy.KeyEnv == "" &&
			policy.KeySecret == "" {
			return errors.New("backup is encrypted, " +
				"but no encryption key (KeyFile, KeyEnv, or KeySecret) was provided in the encryption policy")
		}
	}

	return nil
}

// destinationNamespacesForRestore resolves effective destination namespaces for a restore request.
func destinationNamespacesForRestore(remapping *model.RestoreNamespace, sourceNamespaces []string) []string {
	if remapping != nil && remapping.Destination != nil {
		return []string{*remapping.Destination}
	}

	return sourceNamespaces
}

// sourceNamespacesFromBackups extracts source namespaces from backup metadata entries.
func sourceNamespacesFromBackups(backups []model.BackupDetails) []string {
	namespaces := make([]string, 0, len(backups))
	for _, b := range backups {
		namespaces = append(namespaces, b.Namespace)
	}

	return namespaces
}

// namespacesOverlap reports whether restore and backup namespace scopes intersect.
// It returns the first overlapping namespace found, or an empty string if none.
func namespacesOverlap(restoreNamespaces []string, backupNamespaces []string) string {
	if len(restoreNamespaces) == 0 {
		if len(backupNamespaces) == 0 {
			return "all"
		}
		return backupNamespaces[0]
	}
	// Empty backup namespace list means routine backs up the whole cluster.
	if len(backupNamespaces) == 0 {
		return restoreNamespaces[0]
	}

	for _, ns := range backupNamespaces {
		if slices.Contains(restoreNamespaces, ns) {
			return ns
		}
	}

	return ""
}

func allBackupsEmpty(backups []model.BackupDetails) bool {
	for _, b := range backups {
		if b.FileCount > 0 {
			return false
		}
	}

	return true
}
