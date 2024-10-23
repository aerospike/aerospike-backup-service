package service

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	_ "github.com/aerospike/aerospike-backup-service/v2/modules/schema" // it's required to load configuration schemas in init method
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/aerospike/aerospike-management-lib/info"
	"github.com/aerospike/backup-go"
	"github.com/go-logr/logr"
)

const namespaceInfo = "namespaces"

type NamespaceValidator interface {
	MissingNamespaces(cluster *model.AerospikeCluster, namespaces []string) []string
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

// clusterHasRequiredNamespace checks if given namespace exists in cluster.
func clusterHasRequiredNamespace(namespace string, client backup.AerospikeClient) (bool, error) {
	namespacesInCluster, err := getAllNamespacesOfCluster(client)
	if err != nil {
		return false, fmt.Errorf("failed to retrieve namespaces from cluster: %w", err)
	}

	for _, n := range namespacesInCluster {
		if n == namespace {
			return true, nil
		}
	}

	return false, nil
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
	return strings.Split(namespaces, ";"), nil
}

func getClusterConfiguration(client backup.AerospikeClient) []asconfig.DotConf {
	activeHosts := getActiveHosts(client)

	var outputs = make([]asconfig.DotConf, 0, len(activeHosts))
	policy := client.Cluster().ClientPolicy()
	for _, host := range activeHosts {
		asInfo := info.NewAsInfo(logr.Logger{}, host, &policy)

		conf, err := asconfig.GenerateConf(logr.Discard(), asInfo, true)
		if err != nil {
			slog.Error("Error reading configuration",
				slog.Any("host", host), slog.Any("err", err))
			continue
		}
		asconf, _ := asconfig.NewMapAsConfig(logr.Discard(), conf.Conf)
		configAsString, err := util.TryAndRecover(asconf.ToConfFile)
		if err != nil {
			slog.Error("Error serialising configuration",
				slog.Any("host", host), slog.Any("err", err))
			continue
		}

		outputs = append(outputs, configAsString)
	}

	return outputs
}

func getActiveHosts(client backup.AerospikeClient) []*as.Host {
	var activeHosts []*as.Host
	for _, node := range client.GetNodes() {
		if node.IsActive() {
			activeHosts = append(activeHosts, node.GetHost())
		}
	}

	return activeHosts
}
