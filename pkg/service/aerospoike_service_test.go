//go:build !ci

package service

import (
	"testing"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/smithy-go/ptr"
)

func Test(t *testing.T) {
	cluster := model.AerospikeCluster{
		User:     ptr.String("tester"),
		Password: ptr.String("psw"),
	}
	namespaces, err := getNamespaces(&cluster)
	if err != nil {
		t.Fatalf("Expected error nil, got %v", err)
	}

	if len(namespaces) == 0 {
		t.Fatalf("No namespaces found")
	}
}
