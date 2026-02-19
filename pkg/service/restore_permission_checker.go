package service

import (
	"errors"
	"maps"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

const restoreNotAllowedDuringBackupsMsg = "restore not allowed during backups on the same cluster:namespace. " +
	"Please cancel existing backups jobs to perform restore"

var ErrRestoreNotAllowedDuringBackups = errors.New(restoreNotAllowedDuringBackupsMsg)

// RestorePermissionChecker answers whether restore is allowed for a cluster/namespace set.
type RestorePermissionChecker interface {
	// EnsureAllowedForPathRestore validates restore-by-path against running backups.
	EnsureAllowedForPathRestore(
		cluster *model.AerospikeCluster,
		remapping *model.RestoreNamespace,
		backups []model.BackupDetails,
	) error
	// EnsureAllowedForTimeRestore validates restore-by-time against running backups.
	EnsureAllowedForTimeRestore(
		cluster *model.AerospikeCluster,
		remapping *model.RestoreNamespace,
		backupsByNamespace map[string][]model.BackupDetails,
	) error
}

type restorePermissionChecker struct {
	runningBackups RunningBackupsRegistry
	routines       routineProvider
}

// NewRestorePermissionChecker creates a checker for restore-vs-backup conflicts.
func NewRestorePermissionChecker(
	runningBackups RunningBackupsRegistry,
	routines routineProvider,
) RestorePermissionChecker {
	return &restorePermissionChecker{
		runningBackups: runningBackups,
		routines:       routines,
	}
}

// ensureAllowed blocks restore when an overlapping backup is already running.
func (r *restorePermissionChecker) ensureAllowed(
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

// EnsureAllowedForPathRestore checks restore eligibility for backups found by path.
func (r *restorePermissionChecker) EnsureAllowedForPathRestore(
	cluster *model.AerospikeCluster,
	remapping *model.RestoreNamespace,
	backups []model.BackupDetails,
) error {
	sourceNamespaces := sourceNamespacesFromBackups(backups)
	destinationNamespaces := destinationNamespacesForRestore(remapping, sourceNamespaces)
	return r.ensureAllowed(cluster, destinationNamespaces)
}

// EnsureAllowedForTimeRestore checks restore eligibility for point-in-time backups.
func (r *restorePermissionChecker) EnsureAllowedForTimeRestore(
	cluster *model.AerospikeCluster,
	remapping *model.RestoreNamespace,
	backupsByNamespace map[string][]model.BackupDetails,
) error {
	sourceNamespaces := sourceNamespacesFromBackupsByNamespace(backupsByNamespace)
	destinationNamespaces := destinationNamespacesForRestore(remapping, sourceNamespaces)
	return r.ensureAllowed(cluster, destinationNamespaces)
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
