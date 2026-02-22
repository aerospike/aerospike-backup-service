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
		request *model.RestoreRequest,
		infoGetter backup.InfoGetter,
		backups []model.BackupDetails,
	) error
	// ValidateTimeRestore validates point-in-time restore preconditions.
	ValidateTimeRestore(
		ctx context.Context,
		request *model.RestoreTimestampRequest,
		infoGetter backup.InfoGetter,
		backupsByNamespace map[string][]model.BackupDetails,
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
	request *model.RestoreRequest,
	infoGetter backup.InfoGetter,
	backups []model.BackupDetails,
) error {
	if len(backups) == 0 {
		return nil // no backup metadata found; nothing to validate, will try to restore as-is.
	}

	if allBackupsEmpty(backups) {
		return errors.New("backup metadata indicates there is no data to restore (file count is zero)")
	}

	if err := validateBackupsCreatedAtTheSameTime(backups); err != nil {
		return err
	}

	for _, b := range backups {
		if err := validateBackupEncryption(b, request.Policy.EncryptionPolicy); err != nil {
			return err
		}
	}

	sourceNamespaces := sourceNamespacesFromBackups(backups)

	return r.validateCommon(ctx, request.DestinationCluster, request.Policy.Namespace, sourceNamespaces, infoGetter)
}

// ValidateTimeRestore validates point-in-time restore preconditions.
func (r *restorePreflight) ValidateTimeRestore(
	ctx context.Context,
	request *model.RestoreTimestampRequest,
	infoGetter backup.InfoGetter,
	backupsByNamespace map[string][]model.BackupDetails,
) error {
	for _, backups := range backupsByNamespace {
		for _, b := range backups {
			if err := validateBackupEncryption(b, request.Policy.EncryptionPolicy); err != nil {
				return err
			}
		}
	}

	sourceNamespaces := sourceNamespacesFromBackupsByNamespace(backupsByNamespace)
	return r.validateCommon(ctx, request.DestinationCluster, request.Policy.Namespace, sourceNamespaces, infoGetter)
}

func (r *restorePreflight) validateCommon(
	ctx context.Context,
	cluster *model.AerospikeCluster,
	remapping *model.RestoreNamespace,
	sourceNamespaces []string,
	infoGetter backup.InfoGetter,
) error {
	destinationNamespaces := destinationNamespacesForRestore(remapping, sourceNamespaces)

	if err := validateDestinationNamespaces(ctx, destinationNamespaces, infoGetter); err != nil {
		return err
	}

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

		if overlappingNS := namespacesOverlap(destinationNamespaces, routine.Namespaces); overlappingNS != "" {
			return fmt.Errorf("restore not allowed during backups on cluster %s, namespace %q. "+
				"Please cancel existing backups jobs to perform restore", cluster.ToString(), overlappingNS)
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
	if len(backups) == 0 {
		return nil
	}

	for _, b := range backups {
		if !b.Created.Equal(backups[0].Created) {
			return fmt.Errorf("backups from different times were found: %s and %s",
				b.Created.String(), backups[0].Created.String())
		}
	}

	return nil
}

// validateBackupEncryption validates that the backup encryption matches the provided policy.
func validateBackupEncryption(backup model.BackupDetails, policy *model.EncryptionPolicy) error {
	if backup.Encryption == "" || backup.Encryption == model.EncryptNone {
		return nil
	}
	if policy == nil {
		return fmt.Errorf("backup is encrypted with mode '%s', "+
			"but no encryption policy was provided in the restore request", backup.Encryption)
	}

	if policy.Mode != backup.Encryption {
		return fmt.Errorf("backup is encrypted with mode '%s', "+
			"but the provided encryption policy specifies mode '%s'", backup.Encryption, policy.Mode)
	}
	if policy.KeyFile == nil &&
		policy.KeyEnv == nil &&
		policy.KeySecret == nil {
		return errors.New("backup is encrypted, " +
			"but no encryption key (KeyFile, KeyEnv, or KeySecret) was provided in the encryption policy")
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

// sourceNamespacesFromBackupsByNamespace extracts source namespaces from map keys.
func sourceNamespacesFromBackupsByNamespace(backupsByNamespace map[string][]model.BackupDetails) []string {
	return slices.Collect(maps.Keys(backupsByNamespace))
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

func allBackupsEmpty(backups []model.BackupDetails) bool {
	for _, b := range backups {
		if b.FileCount > 0 {
			return false
		}
	}

	return true
}
