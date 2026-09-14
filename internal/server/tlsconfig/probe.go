package tlsconfig

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	clienttls "github.com/aerospike/aerospike-backup-service/v3/pkg/tlsconfig"
)

// Prober validates TLS material in a service config that is not owned by TLSProvider.
// HTTPS key-pair fail-fast is TLSProvider.Load, not this interface.
type Prober interface {
	// Probe loads the TLS material of every Secret Agent and Aerospike cluster
	// without opening network connections.
	Probe(ctx context.Context, config *model.Config) error
}

type prober struct {
	resolver secrets.ClusterTLSResolver
}

var _ Prober = (*prober)(nil)

// NewProber returns a TLS material prober.
func NewProber(resolver secrets.ClusterTLSResolver) Prober {
	return &prober{resolver: resolver}
}

// Probe loads the TLS material of every Secret Agent and Aerospike cluster
// without opening network connections.
func (p *prober) Probe(ctx context.Context, config *model.Config) error {
	backupConfig := config.BackupConfigCopy()

	for name, agent := range backupConfig.SecretAgents {
		if err := probeSecretAgent(agent); err != nil {
			return fmt.Errorf("secret agent %q TLS validation failed: %w", name, err)
		}
	}

	if https := config.ServiceConfig.ServerHTTPS; https != nil {
		if err := probeSecretAgent(https.SecretAgent); err != nil {
			return fmt.Errorf("secret agent of the HTTPS listener TLS validation failed: %w", err)
		}
	}

	for name, cluster := range backupConfig.AerospikeClusters {
		var agent *model.SecretAgent
		if cluster.Credentials != nil {
			agent = cluster.Credentials.SecretAgent
		}
		if err := probeSecretAgent(agent); err != nil {
			return fmt.Errorf("secret agent of cluster %q TLS validation failed: %w", name, err)
		}
		if err := p.probeCluster(ctx, cluster); err != nil {
			return fmt.Errorf("cluster %q TLS validation failed: %w", name, err)
		}
	}

	return nil
}

// probeSecretAgent verifies that an agent's CA and client key pair can be loaded.
// An agent without TLS files talks plaintext and has nothing to load.
func probeSecretAgent(agent *model.SecretAgent) error {
	if agent == nil || !hasClientTLSFiles(agent.ClientTLS) {
		return nil
	}

	_, err := clienttls.NewTLSConfig(&model.TLS{ClientTLS: agent.ClientTLS})

	return err
}

func hasClientTLSFiles(tls model.ClientTLS) bool {
	return tls.CAFile != "" || tls.Certfile != "" || tls.Keyfile != ""
}

// probeCluster verifies that a cluster's CA and client key pair can be loaded.
// Secret values are resolved into a copy so the model never retains plaintext.
func (p *prober) probeCluster(ctx context.Context, cluster *model.AerospikeCluster) error {
	if cluster == nil || cluster.TLS == nil {
		return nil
	}

	tlsConfig, err := p.resolver.Resolve(ctx, cluster)
	if err != nil {
		return err
	}

	_, err = clienttls.NewTLSConfig(&tlsConfig)

	return err
}
