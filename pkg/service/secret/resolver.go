package secrets

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
)

// Resolver resolves values that may reference secrets.
//
// Resolution is a direct pass-through to backup.ParseSecret; no caching is performed.
// Each resolution constructs a new Secret Agent client (backup.NewSecretAgentClient).
// Where TLS to the agent is configured, that is a TLS handshake per resolution.
type Resolver interface {
	// Resolve resolves the value using the secret agent if configured, otherwise returns the value as is.
	// If the secret agent is nil, the value is returned as is.
	// If resolution fails, the error is returned immediately without fallback; this is fail-closed behavior
	// to prevent silent degradation during Secret Agent outages.
	Resolve(ctx context.Context, agent *model.SecretAgent, value string) (string, error)
}

type resolverImpl struct{}

func NewResolver(ctx context.Context) Resolver {
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
