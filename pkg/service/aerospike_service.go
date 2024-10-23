package service

import (
	"fmt"
	"log/slog"
	"strings"

	_ "github.com/aerospike/aerospike-backup-service/v2/modules/schema" // it's required to load configuration schemas in init method
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/aerospike/aerospike-management-lib/info"
	"github.com/aerospike/backup-go"
	"github.com/go-logr/logr"
)

const namespaceInfo = "namespaces"

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
