//go:build !ci

package service

import (
	"testing"

	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup/pkg/model"
)

func TestNamespaces(t *testing.T) {
	cluster := model.NewLocalAerospikeCluster()
	namespaces, err := getAllNamespacesOfCluster(cluster)
	if err != nil {
		t.Fatalf("Expected error nil, got %v", err)
	}

	if len(namespaces) == 0 {
		t.Fatalf("No namespaces found")
	}
}

func TestVersion(t *testing.T) {
	cluster := model.NewLocalAerospikeCluster()

	client, aerr := as.NewClientWithPolicyAndHost(cluster.ASClientPolicy(), cluster.ASClientHosts()...)
	if aerr != nil {
		t.Fatalf("failed to connect to Aerospike server: %s", aerr)
	}
	defer client.Close()

	v, err := GetAerospikeVersion(client)
	if err != nil {
		t.Fatalf("Expected error nil, got %v", err)
	}

	if v[0] != 7 {
		t.Fatalf("Expected AS version 7, got %d", v[0])
	}
}
