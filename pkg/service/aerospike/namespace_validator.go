package aerospike

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/util/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/pkg/asinfo"
)

// NamespaceValidator provides methods for validating namespace existence in Aerospike clusters.
type NamespaceValidator interface {
	// ValidateRoutines verifies that all namespaces referenced in backup routines
	// for a given cluster exist in that cluster.
	ValidateRoutines(cluster *model.AerospikeCluster, routines map[string]*model.BackupRoutine) error

	// ValidateBackupRoutineNamespaces validates that all namespaces referenced in a
	// backup routine exist in its source cluster.
	ValidateBackupRoutineNamespaces(routine *model.BackupRoutine) error

	// ValidateConfig validates all backup routines in the configuration.
	ValidateConfig(configModel *model.Config) error

	// IsEmpty checks if a namespace in a cluster is empty of records.
	IsEmpty(client *backup.Client, namespace string, request *model.RestoreTimestampRequest) (bool, error)

	// GetAllNamespacesOfCluster retrieves all namespace names from a cluster.
	GetAllNamespacesOfCluster(cluster asinfo.NodeGetter) ([]string, error)

	// ValidatePresent verifies that all provided namespaces exist in the cluster.
	ValidatePresent(cluster *model.AerospikeCluster, ns ...string) error
}

// defaultNamespaceValidator implements the NamespaceValidator interface.
type defaultNamespaceValidator struct {
	clientManager ClientManager
	infoRequest   InfoRequest
}

// NewNamespaceValidator returns a new NamespaceValidator instance.
func NewNamespaceValidator(clientManager ClientManager, asinfo InfoRequest) NamespaceValidator {
	return &defaultNamespaceValidator{
		clientManager: clientManager,
		infoRequest:   asinfo,
	}
}

func (nv *defaultNamespaceValidator) missingNamespaces(
	cluster *model.AerospikeCluster,
	namespaces []string,
) ([]string, error) {
	if len(namespaces) == 0 {
		return nil, nil
	}

	namespacesInCluster, err := nv.getClusterNamespaces(cluster)
	if err != nil {
		return nil, err
	}

	return util.MissingElements(namespaces, namespacesInCluster), nil
}

func (nv *defaultNamespaceValidator) ValidateRoutines(
	cluster *model.AerospikeCluster,
	routines map[string]*model.BackupRoutine,
) error {
	var errs []error
	for routineName, routine := range filterRoutinesByCluster(routines, cluster) {
		missingNamespaces, err := nv.missingNamespaces(cluster, routine.Namespaces)
		if err != nil {
			// if we can't connect to the cluster, we can't validate any routine for it
			return fmt.Errorf("cannot validate routines for cluster: %w", err)
		}
		if len(missingNamespaces) > 0 {
			errs = append(errs, fmt.Errorf("cluster is missing namespaces %v for routine %s",
				missingNamespaces, routineName))
		}
	}

	return errors.Join(errs...)
}

func (nv *defaultNamespaceValidator) ValidateBackupRoutineNamespaces(
	routine *model.BackupRoutine,
) error {
	missingNSs, err := nv.missingNamespaces(routine.SourceCluster, routine.Namespaces)
	if err != nil {
		return err
	}
	if len(missingNSs) > 0 {
		return fmt.Errorf("the following namespaces are missing in the cluster: %v", missingNSs)
	}
	return nil
}

func (nv *defaultNamespaceValidator) ValidateConfig(configModel *model.Config) error {
	for k, v := range configModel.Routines() {
		if err := nv.ValidateBackupRoutineNamespaces(v); err != nil {
			return fmt.Errorf("validation error for routine %s: %w", k, err)
		}
	}
	return nil
}

func (nv *defaultNamespaceValidator) IsEmpty(
	client *backup.Client,
	namespace string,
	request *model.RestoreTimestampRequest,
) (bool, error) {
	count, err := nv.infoRequest.RecordCount(client.AerospikeClient().Cluster(), namespace, request.Policy.SetList)
	if err != nil {
		return false, err
	}

	return count == 0, nil
}

func (nv *defaultNamespaceValidator) GetAllNamespacesOfCluster(
	cluster asinfo.NodeGetter,
) ([]string, error) {
	return nv.infoRequest.Namespaces(cluster)
}

func (nv *defaultNamespaceValidator) ValidatePresent(
	cluster *model.AerospikeCluster,
	namespaces ...string,
) error {
	if len(namespaces) == 0 {
		return nil
	}

	missing, err := nv.missingNamespaces(cluster, namespaces)
	if err != nil {
		return err
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing namespaces: %v", missing)
	}

	return nil
}

// getClusterNamespaces retrieves the list of namespaces from the given cluster.
// It handles client acquisition and release.
func (nv *defaultNamespaceValidator) getClusterNamespaces(
	cluster *model.AerospikeCluster,
) ([]string, error) {
	backupClient, err := nv.clientManager.GetClient(cluster)
	if err != nil {
		slog.Error("Failed to connect to Aerospike cluster", attr.Error(err))
		return nil, fmt.Errorf("failed to connect to Aerospike cluster: %w", err)
	}
	defer nv.clientManager.Close(backupClient)

	namespaces, err := nv.GetAllNamespacesOfCluster(backupClient.AerospikeClient().Cluster())
	if err != nil {
		slog.Error("Failed to retrieve namespaces from cluster", attr.Error(err))
		return nil, err
	}

	return namespaces, nil
}

// filterRoutinesByCluster filters backup routines by the given cluster.
func filterRoutinesByCluster(
	routines map[string]*model.BackupRoutine,
	cluster *model.AerospikeCluster,
) map[string]*model.BackupRoutine {
	filteredRoutines := make(map[string]*model.BackupRoutine)
	for name, routine := range routines {
		if routine.SourceCluster == cluster {
			filteredRoutines[name] = routine
		}
	}
	return filteredRoutines
}
