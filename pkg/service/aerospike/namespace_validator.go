package aerospike

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
)

const namespaceInfo = "namespaces"

type NamespaceValidator interface {
	// MissingNamespaces returns a slice containing any namespaces specified in the
	// provided slice which do not exist on the given cluster.
	MissingNamespaces(cluster *model.AerospikeCluster, namespaces []string) []string
	// ValidateRoutines verifies that all namespaces referenced in backup routines
	// exist in their respective clusters.
	ValidateRoutines(cluster *model.AerospikeCluster, config *model.Config) error
}

type defaultNamespaceValidator struct {
	ClientManager ClientManager
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
		slog.Info("Failed to connect to aerospike cluster", slog.Any("error", err))
		return nil
	}
	defer nv.ClientManager.Close(backupClient)

	namespacesInCluster, err := getAllNamespacesOfCluster(backupClient.AerospikeClient())
	if err != nil {
		slog.Info("Failed to retrieve namespaces from cluster", slog.Any("error", err))
	}

	return util.MissingElements(namespaces, namespacesInCluster)
}

func (nv *defaultNamespaceValidator) ValidateRoutines(cluster *model.AerospikeCluster, config *model.Config) error {
	var err error
	routines := filterRoutinesByCluster(config.BackupRoutines, cluster)
	for routineName, routine := range routines {
		missingNamespaces := nv.MissingNamespaces(cluster, routine.Namespaces)
		if len(missingNamespaces) > 0 {
			err = errors.Join(err, fmt.Errorf("cluster is missing namespaces %v that are used in routine %v",
				missingNamespaces, routineName))
		}
	}

	return err
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
func getAllNamespacesOfCluster(client backup.AerospikeClient) ([]string, error) {
	node, err := client.Cluster().GetRandomNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	infoRes, err := node.RequestInfo(&as.InfoPolicy{}, namespaceInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info: %w", err)
	}
	namespaces := infoRes[namespaceInfo]
	slog.Debug("Retrieved namespace info", "result", namespaces)

	return strings.Split(namespaces, ";"), nil
}

// ResolveNamespaces returns the list of namespaces to back up.
// If `namespaces` is empty, it fetches all namespaces from the cluster via the provided client.
func ResolveNamespaces(namespaces []string, client backup.AerospikeClient) ([]string, error) {
	if len(namespaces) == 0 {
		return getAllNamespacesOfCluster(client)
	}

	return namespaces, nil
}

// NoopNamespaceValidator is a noop implementation of the NamespaceValidator interface.
type NoopNamespaceValidator struct{}

// MissingNamespaces returns an empty slice, indicating no namespaces are missing.
func (n *NoopNamespaceValidator) MissingNamespaces(_ *model.AerospikeCluster, _ []string) []string {
	return nil
}

// ValidateRoutines returns nil, indicating no error in validation.
func (n *NoopNamespaceValidator) ValidateRoutines(_ *model.AerospikeCluster, _ *model.Config) error {
	return nil
}
