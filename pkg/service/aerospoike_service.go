package service

import (
	"fmt"
	"strings"

	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup/pkg/model"
)

const namespaceInfo = "namespaces"

func getNamespaces(cluster *model.AerospikeCluster) ([]string, error) {
	policy := as.NewClientPolicy()
	policy.User = *cluster.User
	policy.Password = *cluster.Password

	// Use the client policy when connecting to Aerospike
	client, err := as.NewClientWithPolicyAndHost(policy, as.NewHost("localhost", 3000))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Aerospike server: %s", err)
	}

	node, err := client.Cluster().GetRandomNode()
	infoRes, err := node.RequestInfo(&as.InfoPolicy{}, namespaceInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info: %s", err)
	}
	namespaces := infoRes[namespaceInfo]
	return strings.Split(namespaces, ";"), nil
}
