package service

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/aerospike/aerospike-management-lib/info"
	_ "github.com/aerospike/backup/modules/schema" // it's required to load configuration schemas in init method
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
	"github.com/go-logr/logr"
)

const namespaceInfo = "namespaces"

func getAllNamespacesOfCluster(cluster *model.AerospikeCluster) ([]string, error) {
	client, err := as.NewClientWithPolicyAndHost(cluster.ASClientPolicy(), cluster.ASClientHosts()...)

	if err != nil {
		return nil, fmt.Errorf("failed to connect to Aerospike server: %s", err)
	}
	defer client.Close()

	node, err := client.Cluster().GetRandomNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %s", err)
	}
	infoRes, err := node.RequestInfo(&as.InfoPolicy{}, namespaceInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info: %s", err)
	}
	namespaces := infoRes[namespaceInfo]
	return strings.Split(namespaces, ";"), nil
}

type AerospikeVersion [4]int

// GetAerospikeVersion gets the version of the Aerospike cluster
func GetAerospikeVersion(client *as.Client) (AerospikeVersion, error) {
	node, aerr := client.Cluster().GetRandomNode()
	if aerr != nil {
		return AerospikeVersion{}, fmt.Errorf("failed to get node: %s", aerr)
	}

	res, aerr := node.RequestInfo(&as.InfoPolicy{}, "version")
	if aerr != nil {
		return AerospikeVersion{}, fmt.Errorf("error during RequestInfo: %w", aerr)
	}

	versionInfo := res["version"]
	parts := strings.Split(versionInfo, " ")
	versionParts := strings.Split(parts[len(parts)-1], ".")
	if len(versionParts) > 4 {
		return AerospikeVersion{}, fmt.Errorf("unexpected vesrion format %s", versionInfo)
	}
	var version AerospikeVersion
	for i, v := range versionParts {
		var err error
		version[i], err = strconv.Atoi(v)
		if err != nil {
			return AerospikeVersion{}, fmt.Errorf("unexpected vesrion format %s: %w", versionInfo, err)
		}
	}
	return version, nil
}

func getClusterConfiguration(cluster *model.AerospikeCluster) ([]asconfig.DotConf, error) {
	activeHosts, err := getActiveHosts(cluster)
	if err != nil {
		return nil, err
	}

	var outputs = make([]asconfig.DotConf, 0, len(activeHosts))
	for _, host := range activeHosts {
		asInfo := info.NewAsInfo(logr.Logger{}, host, cluster.ASClientPolicy())

		conf, err := asconfig.GenerateConf(logr.Discard(), asInfo, true)
		if err != nil {
			slog.Error("Error reading configuration", "host", host, "err", err)
			continue
		}
		asconf, _ := asconfig.NewMapAsConfig(logr.Discard(), conf.Conf)
		configAsString, err := util.TryAndRecover(asconf.ToConfFile)
		if err != nil {
			slog.Error("Error serialising configuration", "host", host, "err", err)
			continue
		}

		outputs = append(outputs, configAsString)
	}

	return outputs, nil
}

func getActiveHosts(cluster *model.AerospikeCluster) ([]*as.Host, error) {
	client, err := as.NewClientWithPolicyAndHost(cluster.ASClientPolicy(), cluster.ASClientHosts()...)
	if err != nil {
		return nil, err
	}
	var activeHosts []*as.Host
	for _, node := range client.GetNodes() {
		if node.IsActive() {
			activeHosts = append(activeHosts, node.GetHost())
		}
	}
	return activeHosts, nil
}
