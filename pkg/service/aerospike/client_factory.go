package aerospike

import (
	"log/slog"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
)

// DefaultClientFactory is the default implementation of ClientFactory.
type DefaultClientFactory struct{}

// NewClientWithPolicyAndHost creates a new Aerospike client with the given policy and hosts.
func (f *DefaultClientFactory) NewClientWithPolicyAndHost(
	cluster *model.AerospikeCluster,
) (backup.AerospikeClient, error) {
	return as.NewClientWithPolicyAndHost(clientPolicy(cluster), clientHosts(cluster)...)
}

// IsClusterHealthy checks if the cluster is connected and responding.
func (f *DefaultClientFactory) IsClusterHealthy(client Cluster) bool {
	if client == nil {
		return false
	}

	cluster := client.Cluster()
	if !cluster.IsConnected() {
		return false
	}

	node, err := cluster.GetRandomNode()
	if err != nil {
		return false
	}

	info, err := node.RequestInfo(as.NewInfoPolicy(), "status")

	return err == nil && info["status"] == "ok"
}

// clientHosts builds and returns a Host list from the AerospikeCluster configuration.
func clientHosts(c *model.AerospikeCluster) []*as.Host {
	hosts := make([]*as.Host, 0, len(c.SeedNodes))
	for _, node := range c.SeedNodes {
		hosts = append(hosts, &as.Host{
			Name:    node.HostName,
			Port:    int(node.Port),
			TLSName: node.TLSName,
		})
	}
	return hosts
}

// clientPolicy builds and returns a new ClientPolicy from the AerospikeCluster configuration.
func clientPolicy(c *model.AerospikeCluster) *as.ClientPolicy {
	policy := as.NewClientPolicy()
	if c.Credentials != nil {
		policy.User = util.ValueOrZero(c.GetUser())
		policy.Password = util.ValueOrZero(c.GetPassword())
		if c.Credentials.AuthMode != nil {
			switch strings.ToUpper(*c.Credentials.AuthMode) {
			case "INTERNAL":
				policy.AuthMode = as.AuthModeInternal
			case "EXTERNAL":
				policy.AuthMode = as.AuthModeExternal
			case "PKI":
				policy.AuthMode = as.AuthModePKI
			}
		}
	}
	if c.ConnTimeout != nil {
		policy.Timeout = *c.ConnTimeout
	}
	if c.UseServicesAlternate != nil {
		policy.UseServicesAlternate = *c.UseServicesAlternate
	}

	var err error
	policy.TlsConfig, err = NewTLSConfig(c.TLS)
	if err != nil {
		slog.Error("Failed to initialize tls.Config",
			slog.String("cluster", util.ValueOrZero(c.ClusterLabel)),
			slog.Any("error", err))
	}

	policy.ConnectionQueueSize = 256
	policy.LimitConnectionsToQueueSize = false

	return policy
}
