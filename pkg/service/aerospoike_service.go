package service

import (
	"fmt"
	"strings"

	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup/pkg/model"
)

const namespaceInfo = "namespaces"

func getAllNamespacesOfCluster(cluster *model.AerospikeCluster) ([]string, error) {
	client, err := as.NewClientWithPolicyAndHost(cluster.ASClientPolicy(), cluster.ASClientHost())
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
