package aerospike

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/util/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/pkg/asinfo"
)

type NamespaceValidator interface {
	// MissingNamespaces returns a slice containing any namespaces specified in the
	// provided slice which do not exist on the given cluster.
	MissingNamespaces(cluster *model.AerospikeCluster, namespaces []string) []string
	// ValidateRoutines verifies that all namespaces referenced in backup routines
	// exist in their respective clusters.
	ValidateRoutines(cluster *model.AerospikeCluster, routines map[string]*model.BackupRoutine) error
	// ValidateBackupRoutineNamespaces validates that all namespaces referenced in the
	// backup routine exist in their respective clusters.
	ValidateBackupRoutineNamespaces(routine *model.BackupRoutine) error
	ValidateConfig(configModel *model.Config) error
	IsEmpty(client *backup.Client, namespace string, request *model.RestoreTimestampRequest) (bool, error)
	GetAllNamespacesOfCluster(cluster asinfo.NodeGetter) ([]string, error)
	ValidatePresent(cluster *model.AerospikeCluster, ns ...string) error
}

type defaultNamespaceValidator struct {
	ClientManager ClientManager
}

func (nv *defaultNamespaceValidator) IsEmpty(
	client *backup.Client,
	namespace string,
	request *model.RestoreTimestampRequest,
) (bool, error) {
	newClient, err := asinfo.NewClient(client.AerospikeClient().Cluster(), as.NewInfoPolicy(), request.Policy.RetryPolicy)
	if err != nil {
		return false, err
	}

	count, err := newClient.GetRecordCount(namespace, request.Policy.SetList)
	if err != nil {
		return false, err
	}

	return count == 0, nil
}

func (nv *defaultNamespaceValidator) ValidateConfig(configModel *model.Config) error {
	for k, v := range configModel.Routines() {
		if err := nv.ValidateBackupRoutineNamespaces(v); err != nil {
			return fmt.Errorf("validation error for routine %s: %w", k, err)
		}
	}

	return nil
}

func NewNamespaceValidator(clientManager ClientManager) NamespaceValidator {
	return &defaultNamespaceValidator{
		ClientManager: clientManager,
	}
}

func (nv *defaultNamespaceValidator) MissingNamespaces(
	cluster *model.AerospikeCluster,
	namespaces []string,
) []string {
	if len(namespaces) == 0 {
		return nil
	}

	backupClient, err := nv.ClientManager.GetClient(cluster)
	if err != nil {
		slog.Info("Failed to connect to aerospike cluster", attr.Error(err))
		return nil
	}
	defer nv.ClientManager.Close(backupClient)

	namespacesInCluster, err := nv.GetAllNamespacesOfCluster(backupClient.AerospikeClient().Cluster())
	if err != nil {
		slog.Info("Failed to retrieve namespaces from cluster", attr.Error(err))
	}

	return util.MissingElements(namespaces, namespacesInCluster)
}

func (nv *defaultNamespaceValidator) ValidateRoutines(
	cluster *model.AerospikeCluster, routines map[string]*model.BackupRoutine,
) error {
	var err error
	for routineName, routine := range filterRoutinesByCluster(routines, cluster) {
		missingNamespaces := nv.MissingNamespaces(cluster, routine.Namespaces)
		if len(missingNamespaces) > 0 {
			err = errors.Join(err, fmt.Errorf("cluster is missing namespaces %v that are used in routine %v",
				missingNamespaces, routineName))
		}
	}

	return err
}

func (nv *defaultNamespaceValidator) ValidateBackupRoutineNamespaces(routine *model.BackupRoutine) error {
	missingNSs := nv.MissingNamespaces(routine.SourceCluster, routine.Namespaces)
	if len(missingNSs) > 0 {
		return fmt.Errorf("the following namespaces are missing in the cluster: %v", missingNSs)
	}
	return nil
}

// filterRoutinesByCluster filters backup routines by the given cluster.
func filterRoutinesByCluster(
	routines map[string]*model.BackupRoutine, cluster *model.AerospikeCluster,
) map[string]*model.BackupRoutine {
	filteredRoutines := make(map[string]*model.BackupRoutine)
	for name, routine := range routines {
		if routine.SourceCluster == cluster {
			filteredRoutines[name] = routine
		}
	}
	return filteredRoutines
}

func (nv *defaultNamespaceValidator) GetAllNamespacesOfCluster(cluster asinfo.NodeGetter) ([]string, error) {
	newClient, err := asinfo.NewClient(cluster, as.NewInfoPolicy(), nil)
	if err != nil {
		return nil, err
	}

	return newClient.GetNamespacesList()
}

// ValidatePresent verifies that all provided namespaces exist.
func (nv *defaultNamespaceValidator) ValidatePresent(cluster *model.AerospikeCluster, namespaces ...string) error {
	if len(namespaces) == 0 {
		return nil
	}

	backupClient, err := nv.ClientManager.GetClient(cluster)
	if err != nil {
		slog.Info("Failed to connect to aerospike cluster", attr.Error(err))
		return nil
	}
	defer nv.ClientManager.Close(backupClient)

	clusterNS, err := nv.GetAllNamespacesOfCluster(backupClient.AerospikeClient().Cluster())
	if err != nil {
		return err
	}

	missing := util.MissingElements(namespaces, clusterNS)
	if len(missing) > 0 {
		return fmt.Errorf("missing namespaces: %v", missing)
	}

	return nil
}
