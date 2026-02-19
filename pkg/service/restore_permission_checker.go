package service

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
)

const restoreNotAllowedDuringBackupsMsg = "restore not allowed during backups on the same cluster:namespace. " +
	"Please cancel existing backups jobs to perform restore"

var ErrRestoreNotAllowedDuringBackups = errors.New(restoreNotAllowedDuringBackupsMsg)

// RestorePreflight validates restore preconditions before actual execution starts.
type RestorePreflight interface {
	// ValidatePathRestore validates path-restore preconditions.
	ValidatePathRestore(
		ctx context.Context,
		cluster *model.AerospikeCluster,
		policy *model.RestorePolicy,
		infoGetter backup.InfoGetter,
		backups []model.BackupDetails,
	) error
	// ValidateTimeRestore validates point-in-time restore preconditions.
	ValidateTimeRestore(
		ctx context.Context,
		cluster *model.AerospikeCluster,
		policy *model.RestorePolicy,
		infoGetter backup.InfoGetter,
		backupsByNamespace map[string][]model.BackupDetails,
		request *model.RestoreTimestampRequest,
	) error
}

type restorePreflight struct {
	runningBackups RunningBackupsRegistry
	routines       routineProvider
}

// NewRestorePreflight creates a preflight validator for restore operations.
func NewRestorePreflight(
	runningBackups RunningBackupsRegistry,
	routines routineProvider,
) RestorePreflight {
	return &restorePreflight{
		runningBackups: runningBackups,
		routines:       routines,
	}
}

// ValidatePathRestore validates path-restore preconditions.
func (r *restorePreflight) ValidatePathRestore(
	ctx context.Context,
	cluster *model.AerospikeCluster,
	policy *model.RestorePolicy,
	infoGetter backup.InfoGetter,
	backups []model.BackupDetails,
) error {
	var remapping *model.RestoreNamespace
	if policy != nil {
		remapping = policy.Namespace
	}
	if err := validateBackupsCreatedAtTheSameTime(backups); err != nil {
		return err
	}
	if err := validateDestinationNamespace(ctx, policy, infoGetter); err != nil {
		return err
	}

	return r.ensureAllowedForPathRestore(cluster, remapping, backups)
}

// ValidateTimeRestore validates point-in-time restore preconditions.
func (r *restorePreflight) ValidateTimeRestore(
	ctx context.Context,
	cluster *model.AerospikeCluster,
	policy *model.RestorePolicy,
	infoGetter backup.InfoGetter,
	backupsByNamespace map[string][]model.BackupDetails,
	request *model.RestoreTimestampRequest,
) error {
	var remapping *model.RestoreNamespace
	if policy != nil {
		remapping = policy.Namespace
	}
	if err := validateEncryption(backupsByNamespace, request); err != nil {
		return err
	}
	if err := validateDestinationNamespace(ctx, policy, infoGetter); err != nil {
		return err
	}

	return r.ensureAllowedForTimeRestore(cluster, remapping, backupsByNamespace)
}

// ensureAllowedForPathRestore checks restore eligibility for backups found by path.
func (r *restorePreflight) ensureAllowedForPathRestore(
	cluster *model.AerospikeCluster,
	remapping *model.RestoreNamespace,
	backups []model.BackupDetails,
) error {
	sourceNamespaces := sourceNamespacesFromBackups(backups)
	destinationNamespaces := destinationNamespacesForRestore(remapping, sourceNamespaces)
	return r.ensureAllowed(cluster, destinationNamespaces)
}

// ensureAllowedForTimeRestore checks restore eligibility for point-in-time backups.
func (r *restorePreflight) ensureAllowedForTimeRestore(
	cluster *model.AerospikeCluster,
	remapping *model.RestoreNamespace,
	backupsByNamespace map[string][]model.BackupDetails,
) error {
	sourceNamespaces := sourceNamespacesFromBackupsByNamespace(backupsByNamespace)
	destinationNamespaces := destinationNamespacesForRestore(remapping, sourceNamespaces)
	return r.ensureAllowed(cluster, destinationNamespaces)
}

// ensureAllowed blocks restore when an overlapping backup is already running.
func (r *restorePreflight) ensureAllowed(
	cluster *model.AerospikeCluster,
	destinationNamespaces []string,
) error {
	if cluster == nil {
		return nil
	}

	runningState := r.runningBackups.GetRunningState()
	if len(runningState) == 0 {
		return nil
	}

	routines := r.routines.Routines()

	for routineName, routineState := range runningState {
		if routineState == nil || (routineState.Full == nil && routineState.Incremental == nil) {
			continue
		}

		routine, found := routines[routineName]
		if !found || routine == nil || routine.SourceCluster == nil {
			continue
		}

		if !clustersMatch(routine.SourceCluster, cluster) {
			continue
		}

		if namespacesOverlap(destinationNamespaces, routine.Namespaces) {
			return ErrRestoreNotAllowedDuringBackups
		}
	}

	return nil
}

// validateDestinationNamespace checks destination namespace existence in destination cluster.
func validateDestinationNamespace(
	ctx context.Context,
	policy *model.RestorePolicy,
	infoGetter backup.InfoGetter,
) error {
	if policy == nil {
		return nil
	}
	if policy.Namespace == nil {
		return nil
	}

	destinationNS := *policy.Namespace.Destination
	namespaces, err := infoGetter.GetNamespacesList(ctx)
	if err != nil {
		return fmt.Errorf("failed to get namespaces from destination cluster: %w", err)
	}
	if !slices.Contains(namespaces, destinationNS) {
		return fmt.Errorf("destination cluster does not have required namespace: %s", destinationNS)
	}

	return nil
}

// validateBackupsCreatedAtTheSameTime ensures all selected backups belong to the same snapshot.
func validateBackupsCreatedAtTheSameTime(backups []model.BackupDetails) error {
	if len(backups) == 0 {
		return nil
	}

	for _, backup := range backups {
		if backup.Created != backups[0].Created {
			return fmt.Errorf("backups from different times were found: %s and %s",
				backup.Created.String(), backups[0].Created.String())
		}
	}

	return nil
}

// validateEncryption validates restore encryption policy against backup metadata.
func validateEncryption(
	backupsByNamespace map[string][]model.BackupDetails,
	request *model.RestoreTimestampRequest,
) error {
	for _, backups := range backupsByNamespace {
		for _, backup := range backups {
			if backup.Encryption == "" || backup.Encryption == model.EncryptNone {
				continue
			}
			if request.Policy == nil || request.Policy.EncryptionPolicy == nil {
				return fmt.Errorf("backup is encrypted with mode '%s', "+
					"but no encryption policy was provided in the restore request", backup.Encryption)
			}

			userEncryptionPolicy := request.Policy.EncryptionPolicy
			if userEncryptionPolicy.Mode != backup.Encryption {
				return fmt.Errorf("backup is encrypted with mode '%s', "+
					"but the provided encryption policy specifies mode '%s'", backup.Encryption, userEncryptionPolicy.Mode)
			}
			if userEncryptionPolicy.KeyFile == nil &&
				userEncryptionPolicy.KeyEnv == nil &&
				userEncryptionPolicy.KeySecret == nil {
				return errors.New("backup is encrypted, " +
					"but no encryption key (KeyFile, KeyEnv, or KeySecret) was provided in the encryption policy")
			}
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
	for _, backup := range backups {
		if backup.Namespace == "" {
			continue
		}
		namespaces = append(namespaces, backup.Namespace)
	}

	return namespaces
}

// sourceNamespacesFromBackupsByNamespace extracts source namespaces from map keys.
func sourceNamespacesFromBackupsByNamespace(backupsByNamespace map[string][]model.BackupDetails) []string {
	return slices.Collect(maps.Keys(backupsByNamespace))
}

// namespacesOverlap reports whether restore and backup namespace scopes intersect.
func namespacesOverlap(restoreNamespaces []string, backupNamespaces []string) bool {
	// Empty restore namespace set means "unknown/all namespaces".
	restoreNamespaces = slices.DeleteFunc(slices.Clone(restoreNamespaces), func(namespace string) bool {
		return namespace == ""
	})
	if len(restoreNamespaces) == 0 {
		return true
	}
	// Empty backup namespace list means routine backs up the whole cluster.
	if len(backupNamespaces) == 0 {
		return true
	}

	return slices.ContainsFunc(backupNamespaces, func(namespace string) bool {
		return slices.Contains(restoreNamespaces, namespace)
	})
}

// clustersMatch compares clusters by label first and by full hash as fallback.
func clustersMatch(first *model.AerospikeCluster, second *model.AerospikeCluster) bool {
	if first == nil || second == nil {
		return false
	}

	firstLabel := ptr.ValueOrZero(first.ClusterLabel)
	secondLabel := ptr.ValueOrZero(second.ClusterLabel)
	if firstLabel != "" && secondLabel != "" {
		return firstLabel == secondLabel
	}

	return first.Hash() == second.Hash()
}
