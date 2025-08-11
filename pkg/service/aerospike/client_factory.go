package aerospike

import (
	"log/slog"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/internal/util/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
)

// DefaultClientFactory is the default implementation of ClientFactory.
type DefaultClientFactory struct {
	asinfo InfoRequest
}

func NewClientFactory(asinfo InfoRequest) *DefaultClientFactory {
	return &DefaultClientFactory{
		asinfo: asinfo,
	}
}

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

	if !client.Cluster().IsConnected() {
		return false
	}

	status, err := f.asinfo.Status(client.Cluster())

	return err == nil && status == "ok"
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

	setTLSConfig(c, policy)

	policy.ConnectionQueueSize = 256
	policy.LimitConnectionsToQueueSize = false

	return policy
}

func setTLSConfig(c *model.AerospikeCluster, policy *as.ClientPolicy) {
	if !anySeedNodeHasTLSName(c) {
		if c.TLS != nil {
			slog.Warn("A TLS configuration is provided, but no seed nodes have TLS names. Ignoring TLS settings.",
				slog.String("cluster", util.ValueOrZero(c.ClusterLabel)))
		}

		return // no TLS configuration needed for this cluster
	}

	// Seed nodes require TLS, so a TLS configuration is necessary.
	// If no specific TLS configuration is provided, a default one is used.
	tlsToApply := c.TLS
	if tlsToApply == nil {
		tlsToApply = &model.TLS{}
	}

	var err error
	policy.TlsConfig, err = NewTLSConfig(tlsToApply)
	if err != nil {
		slog.Error("Failed to initialize tls.Config",
			slog.String("cluster", util.ValueOrZero(c.ClusterLabel)),
			attr.Error(err))
	}
}

// anySeedNodeHasTLSName checks if any of the seed nodes are configured with a TLS name.
func anySeedNodeHasTLSName(c *model.AerospikeCluster) bool {
	for _, node := range c.SeedNodes {
		if node.TLSName != "" {
			return true
		}
	}

	return false
}
