//go:build !ci

package aerospike

import (
	"testing"

	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	client, aerr := as.NewClientWithPolicyAndHost(testPolicy(), testHosts()...)
	assert.NoError(t, aerr)
	defer client.Close()

	namespaces, err := getAllNamespacesOfCluster(client)
	assert.NoError(t, err)
	assert.NotEmpty(t, namespaces)
}
