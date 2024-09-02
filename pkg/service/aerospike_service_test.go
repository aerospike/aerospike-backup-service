//go:build !ci

package service

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	cluster := model.NewLocalAerospikeCluster()
	client, aerr := as.NewClientWithPolicyAndHost(cluster.ASClientPolicy(), cluster.ASClientHosts()...)
	assert.NoError(t, aerr)
	namespaces, err := getAllNamespacesOfCluster(client)
	assert.NoError(t, err)
	assert.NotEmpty(t, namespaces)
}
