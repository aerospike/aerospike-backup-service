package secrets

import (
	"context"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
)

type cacheKey struct {
	agent *model.SecretAgent
	value string
}

// Resolver resolves values that may reference secrets.
//
// Implementations may cache resolved secrets to avoid repeated calls
// to external secret backends.
type Resolver interface {
	// Resolve resolves the value using the secret agent if configured, otherwise returns the value as is.
	Resolve(agent *model.SecretAgent, value string) (string, error)
}

type resolverImpl struct {
	cache collections.Cache[cacheKey, string]
}

const storageDuration = 24 * time.Hour // defines how long to store cached secrets before re-reading.

func NewResolver(ctx context.Context) Resolver {
	load := func(key cacheKey) (string, error) {
		agentConfig := key.agent.ToSecretAgentConfig()
		return backup.ParseSecret(agentConfig, key.value)
	}

	return &resolverImpl{
		cache: collections.NewLoadingCache(ctx, load, ptr.Of(storageDuration)),
	}
}

// Resolve resolves the value using the secret agent if configured, otherwise returns the value as is.
func (m *resolverImpl) Resolve(agent *model.SecretAgent, value string) (string, error) {
	if agent == nil {
		return value, nil
	}

	return m.cache.Get(cacheKey{
		agent: agent,
		value: value,
	})
}
