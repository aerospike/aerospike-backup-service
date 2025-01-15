//go:build !ci

package aerospike

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	cluster := model.NewLocalAerospikeCluster()
	client, aerr := as.NewClientWithPolicyAndHost(cluster.ASClientPolicy(), cluster.ASClientHosts()...)
	assert.NoError(t, aerr)
	defer client.Close()

	namespaces, err := getAllNamespacesOfCluster(client)
	assert.NoError(t, err)
	assert.NotEmpty(t, namespaces)
}
