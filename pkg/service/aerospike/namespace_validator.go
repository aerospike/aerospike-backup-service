package aerospike

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/internal/util/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/pkg/asinfo"
)

const namespaceInfo = "namespaces"

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

	namespacesInCluster, err := getAllNamespacesOfCluster(backupClient.AerospikeClient())
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

// getAllNamespacesOfCluster retrieves a list of all namespaces in an Aerospike cluster.
func getAllNamespacesOfCluster(client Cluster) ([]string, error) {
	node, err := client.Cluster().GetRandomNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	infoRes, err := node.RequestInfo(&as.InfoPolicy{}, namespaceInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info: %w", err)
	}
	namespaces := infoRes[namespaceInfo]
	slog.Debug("Retrieved namespace info", slog.String("result", namespaces))

	return strings.Split(namespaces, ";"), nil
}

// ResolveNamespaces returns the list of namespaces to back up.
// If `namespaces` is empty, it fetches all namespaces from the cluster via the provided client.
func ResolveNamespaces(namespaces []string, client Cluster) ([]string, error) {
	if len(namespaces) == 0 {
		return getAllNamespacesOfCluster(client)
	}

	return namespaces, nil
}
