//go:build !ci

package service

import (
	"testing"

	"github.com/aerospike/backup/pkg/model"
)

func Test(t *testing.T) {
	cluster := model.NewLocalAerospikeCluster()
	namespaces, err := getAllNamespacesOfCluster(cluster)
	if err != nil {
		t.Fatalf("Expected error nil, got %v", err)
	}

	if len(namespaces) == 0 {
		t.Fatalf("No namespaces found")
	}
}
