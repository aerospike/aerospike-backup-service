package secrets

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// KeyfilePasswordResolver resolves a TLS config's KeyfilePassword when it holds a
// Secret Agent reference rather than a literal value (see dto.TLS.KeyfilePassword).
type KeyfilePasswordResolver interface {
	// Resolve returns a copy of tlsConfig with KeyfilePassword resolved through
	// agent. An empty password is returned unchanged without contacting the agent.
	//
	// Callers must run this before tlsconfig.NewTLSConfig, which uses
	// KeyfilePassword verbatim as the decryption password: skipping this step
	// passes the literal reference string to the decryptor, which fails every
	// time a Secret Agent reference is used.
	Resolve(ctx context.Context, tlsConfig model.TLS, agent *model.SecretAgent) (model.TLS, error)
}

type keyfilePasswordResolver struct {
	resolver Resolver
}

// NewKeyfilePasswordResolver returns a KeyfilePasswordResolver that uses the
// provided Resolver to resolve Secret Agent-backed values.
func NewKeyfilePasswordResolver(resolver Resolver) KeyfilePasswordResolver {
	return &keyfilePasswordResolver{resolver: resolver}
}

// Resolve returns a copy of tlsConfig with KeyfilePassword resolved through agent.
func (r *keyfilePasswordResolver) Resolve(
	ctx context.Context, tlsConfig model.TLS, agent *model.SecretAgent,
) (model.TLS, error) {
	if tlsConfig.KeyfilePassword == "" {
		return tlsConfig, nil
	}

	password, err := r.resolver.Resolve(ctx, agent, tlsConfig.KeyfilePassword)
	if err != nil {
		return model.TLS{}, fmt.Errorf("failed to resolve TLS key-file-password: %w", err)
	}
	tlsConfig.KeyfilePassword = password

	return tlsConfig, nil
}
