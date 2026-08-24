package secrets

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
)

// Resolver resolves a configuration value that may reference a Secret Agent.
// Nothing is cached: every call goes to the agent, so an agent outage fails the call
// instead of reusing a previously resolved value.
type Resolver interface {
	// Resolve resolves the value using the secret agent if configured, otherwise returns the value as is.
	// If the secret agent is nil, the value is returned as is.
	Resolve(ctx context.Context, agent *model.SecretAgent, value string) (string, error)
}

type resolver struct{}

func NewResolver() Resolver {
	return &resolver{}
}

// Resolve resolves the value using the secret agent if configured, otherwise returns the value as is.
func (m *resolver) Resolve(ctx context.Context, agent *model.SecretAgent, value string) (string, error) {
	if agent == nil {
		return value, nil
	}

	agentConfig := agent.ToSecretAgentConfig()
	return backup.ParseSecret(ctx, agentConfig, value)
}
