package secrets

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// ClusterTLSResolver produces a cluster's TLS configuration with its secret values
// resolved, so callers never have to know where a cluster keeps them.
type ClusterTLSResolver interface {
	// Resolve returns the cluster's TLS configuration with KeyfilePassword resolved
	// through the cluster's Secret Agent, in case it holds a reference rather than a
	// literal value (see dto.TLS.KeyfilePassword). A cluster with no TLS block yields
	// the zero value, which builds a default TLS configuration. On error the returned
	// model.TLS is the zero value and must not be used; the cluster is never modified.
	//
	// Callers must run this before tlsconfig.NewTLSConfig, which uses KeyfilePassword
	// verbatim as the decryption password: skipping it passes the literal reference
	// string to the decryptor, which fails every time a reference is used.
	Resolve(ctx context.Context, cluster *model.AerospikeCluster) (model.TLS, error)
}

type clusterTLSResolver struct {
	resolver Resolver
}

// NewClusterTLSResolver returns a ClusterTLSResolver that uses the provided Resolver
// to resolve Secret Agent-backed values.
func NewClusterTLSResolver(resolver Resolver) ClusterTLSResolver {
	return &clusterTLSResolver{resolver: resolver}
}

// Resolve returns the cluster's TLS configuration with its secret values resolved.
func (r *clusterTLSResolver) Resolve(
	ctx context.Context, cluster *model.AerospikeCluster,
) (model.TLS, error) {
	if cluster == nil || cluster.TLS == nil {
		return model.TLS{}, nil
	}

	tlsConfig := *cluster.TLS
	if tlsConfig.KeyfilePassword == "" {
		return tlsConfig, nil
	}

	var agent *model.SecretAgent
	if cluster.Credentials != nil {
		agent = cluster.Credentials.SecretAgent
	}

	password, err := r.resolver.Resolve(ctx, agent, tlsConfig.KeyfilePassword)
	if err != nil {
		return model.TLS{}, fmt.Errorf("failed to resolve TLS key-file-password: %w", err)
	}
	tlsConfig.KeyfilePassword = password

	return tlsConfig, nil
}
