package secrets

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
)

// Resolver resolves values that may reference secrets.
//
// The resolver is a stateless pass-through to backup.ParseSecret with no internal caching.
// A Secret Agent outage during resolution fails the call (fail-closed; no stale-secret fallback).
// Each resolution creates a new Secret Agent client inside ParseSecret.
type Resolver interface {
	// Resolve resolves the value using the secret agent if configured, otherwise returns the value as is.
	// If the secret agent is nil, the value is returned as is.
	Resolve(ctx context.Context, agent *model.SecretAgent, value string) (string, error)
}

type resolverImpl struct{}

func NewResolver() Resolver {
	return &resolverImpl{}
}

// Resolve resolves the value using the secret agent if configured, otherwise returns the value as is.
func (m *resolverImpl) Resolve(ctx context.Context, agent *model.SecretAgent, value string) (string, error) {
	if agent == nil {
		return value, nil
	}

	agentConfig := agent.ToSecretAgentConfig()
	return backup.ParseSecret(ctx, agentConfig, value)
}
