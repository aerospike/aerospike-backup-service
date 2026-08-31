package tlsconfig

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	clienttls "github.com/aerospike/aerospike-backup-service/v3/pkg/tlsconfig"
)

// Prober validates TLS material in a service config that is not owned by Reloader.
// HTTPS key-pair fail-fast is Reloader.Load, not this interface.
type Prober interface {
	// Probe loads the TLS material of every Secret Agent and Aerospike cluster
	// without opening network connections.
	Probe(ctx context.Context, config *model.Config) error
}

type prober struct {
	resolver secrets.Resolver
}

var _ Prober = (*prober)(nil)

// NewProber returns a TLS material prober.
func NewProber(resolver secrets.Resolver) Prober {
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
		if err := p.probeCluster(ctx, cluster, agent); err != nil {
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
func (p *prober) probeCluster(
	ctx context.Context,
	cluster *model.AerospikeCluster,
	agent *model.SecretAgent,
) error {
	if cluster == nil || cluster.TLS == nil {
		return nil
	}

	tlsConfig := *cluster.TLS
	if tlsConfig.KeyfilePassword != "" {
		password, err := p.resolver.Resolve(ctx, agent, tlsConfig.KeyfilePassword)
		if err != nil {
			return fmt.Errorf("failed to resolve key-file-password: %w", err)
		}
		tlsConfig.KeyfilePassword = password
	}

	if _, err := clienttls.NewTLSConfig(&tlsConfig); err != nil {
		return err
	}

	return nil
}
