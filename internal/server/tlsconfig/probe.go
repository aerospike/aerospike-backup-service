package tlsconfig

import (
	"context"
	"fmt"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	clienttls "github.com/aerospike/aerospike-backup-service/v3/pkg/tlsconfig"
)

// ProbeConfig loads all configured TLS material without starting listeners or watchers.
func ProbeConfig(ctx context.Context, config *model.Config, resolver secrets.Resolver) error {
	if config == nil {
		return nil
	}

	if err := ProbeHTTPS(ctx, config.ServiceConfig.ServerHTTPS, resolver); err != nil {
		return err
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
		if cluster != nil && cluster.Credentials != nil {
			agent = cluster.Credentials.SecretAgent
		}
		if err := ProbeCluster(ctx, cluster, agent, resolver); err != nil {
			return fmt.Errorf("cluster %q TLS validation failed: %w", name, err)
		}
	}

	return nil
}

// ProbeHTTPS verifies that the configured HTTPS key pair and client CA can be loaded.
func ProbeHTTPS(ctx context.Context, config *model.ServerConfigHTTPS, resolver secrets.Resolver) error {
	if config == nil || config.CertFile == "" || config.KeyFile == "" {
		return nil
	}

	if _, err := LoadKeyPair(ctx, config, resolver); err != nil {
		return fmt.Errorf("HTTPS TLS validation failed: %w", err)
	}
	if _, err := NewTLSConfig(config, nil); err != nil {
		return fmt.Errorf("HTTPS TLS validation failed: %w", err)
	}

	return nil
}

// ProbeCluster verifies that a cluster's CA and client key pair can be loaded.
// Secret values are resolved into a copy so the model never retains plaintext.
func ProbeCluster(
	ctx context.Context,
	cluster *model.AerospikeCluster,
	agent *model.SecretAgent,
	resolver secrets.Resolver,
) error {
	if cluster == nil || cluster.TLS == nil {
		return nil
	}

	tlsConfig := *cluster.TLS
	if tlsConfig.KeyfilePassword != "" {
		password, err := resolver.Resolve(ctx, agent, tlsConfig.KeyfilePassword)
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
