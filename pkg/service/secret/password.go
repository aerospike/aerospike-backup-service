package secrets

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/safepath"
)

// PasswordResolver derives an Aerospike password from credentials: a literal value,
// a password file, or a Secret Agent reference resolved through [Resolver].
type PasswordResolver interface {
	// Resolve resolves the password from the credentials.
	Resolve(ctx context.Context, creds *model.Credentials) (*string, error)
}

type passwordResolver struct {
	resolver Resolver
}

// NewPasswordResolver returns a PasswordResolver that uses the provided Resolver
// to resolve Secret Agent-backed values. Passing nil disables Secret Agent
// resolution (password is treated as a literal).
func NewPasswordResolver(resolver Resolver) PasswordResolver {
	return &passwordResolver{resolver: resolver}
}

// Resolve resolves the password from the credentials.
// It handles reading from file or using the Secret Agent.
func (r passwordResolver) Resolve(ctx context.Context, creds *model.Credentials) (*string, error) {
	if creds == nil {
		return nil, nil
	}

	// 1) Resolve Path (file)
	if creds.PasswordPath != "" {
		data, err := safepath.ReadFile(creds.PasswordPath)
		if err != nil {
			return nil, errors.New("failed to read password from password-path")
		}
		slog.Debug("Successfully read password from password-path")

		// Strip only line-ending characters to preserve meaningful spaces.
		password := strings.TrimRight(string(data), "\r\n")

		return &password, nil
	}

	// 2) Resolve (literal or secret-agent reference)
	if creds.Password != "" {
		password, err := r.resolver.Resolve(ctx, creds.SecretAgent, creds.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to read password from secret agent: %w", err)
		}

		return &password, nil
	}

	return nil, errors.New("no valid authentication method configured")
}
