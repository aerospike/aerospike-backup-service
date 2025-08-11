//go:build !ci

package aerospike

import (
	"testing"

	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/stretchr/testify/assert"
)

func testPolicy() *as.ClientPolicy {
	policy := as.NewClientPolicy()
	policy.User = "tester"
	policy.Password = "psw"
	return policy
}

func testHosts() []*as.Host {
	return []*as.Host{
		{Name: "localhost", Port: 3000},
	}
}

func Test(t *testing.T) {
	client, aerr := as.NewClientWithPolicyAndHost(testPolicy(), testHosts()...)
	assert.NoError(t, aerr)
	defer client.Close()

	validator := NewNamespaceValidator(nil)
	namespaces, err := validator.GetAllNamespacesOfCluster(client.Cluster())
	assert.NoError(t, err)
	assert.NotEmpty(t, namespaces)
}
