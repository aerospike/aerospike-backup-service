package tlsconfig

import (
	"context"
	"fmt"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	clienttls "github.com/aerospike/aerospike-backup-service/v3/pkg/tlsconfig"
)

// Prober validates TLS material needed by a running service.
type Prober interface {
	// ProbeConfig validates HTTPS and Aerospike cluster TLS material in a service config.
	ProbeConfig(ctx context.Context, config *model.Config) error
}

type prober struct {
	resolver secrets.Resolver
}

var _ Prober = (*prober)(nil)

// NewProber returns a TLS material prober.
func NewProber(resolver secrets.Resolver) Prober {
	return &prober{resolver: resolver}
}

// ProbeConfig loads all configured TLS material without starting listeners or watchers.
func (p *prober) ProbeConfig(ctx context.Context, config *model.Config) error {
	if config.ServiceConfig.ServerHTTPS != nil {
		if err := p.probeHTTPS(ctx, config.ServiceConfig.ServerHTTPS); err != nil {
			return err
		}
	}

	backupConfig := config.BackupConfigCopy()
	names := make([]string, 0, len(backupConfig.AerospikeClusters))
	for name := range backupConfig.AerospikeClusters {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		cluster := backupConfig.AerospikeClusters[name]
		var agent *model.SecretAgent
		if cluster.Credentials != nil {
			agent = cluster.Credentials.SecretAgent
		}
		if err := p.probeCluster(ctx, cluster, agent); err != nil {
			return fmt.Errorf("cluster %q TLS validation failed: %w", name, err)
		}
	}

	return nil
}

// probeHTTPS verifies that the configured HTTPS key pair and client CA can be loaded.
// Disabled listeners are not probed: those files are unused until HTTPS is enabled.
func (p *prober) probeHTTPS(ctx context.Context, config *model.ServerConfigHTTPS) error {
	if config.Disabled || config.CertFile == "" || config.KeyFile == "" {
		return nil
	}

	if _, err := LoadKeyPair(ctx, config, p.resolver); err != nil {
		return fmt.Errorf("HTTPS TLS validation failed: %w", err)
	}
	if _, err := NewTLSConfig(config, nil); err != nil {
		return fmt.Errorf("HTTPS TLS validation failed: %w", err)
	}

	return nil
}

// probeCluster verifies that a cluster's CA and client key pair can be loaded.
// Secret values are resolved into a copy so the model never retains plaintext.
func (p *prober) probeCluster(
	ctx context.Context,
	cluster *model.AerospikeCluster,
	agent *model.SecretAgent,
) error {
	if cluster.TLS == nil {
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
