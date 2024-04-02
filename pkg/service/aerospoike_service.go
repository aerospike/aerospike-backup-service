package service

import (
	"fmt"
	"log/slog"
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

func getClusterConfiguration(cluster *model.AerospikeCluster) []asconfig.DotConf {
	var outputs = make([]asconfig.DotConf, 0, len(cluster.ASClientHosts()))
	cp := cluster.ASClientPolicy()

	for _, host := range cluster.ASClientHosts() {
		asInfo := info.NewAsInfo(logr.Logger{}, host, cp)

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

	return outputs
}
