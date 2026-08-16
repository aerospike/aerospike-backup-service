package secrets

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// PasswordResolver resolves password from either a file, a literal value,
// or via a Secret Agent.
type PasswordResolver interface {
	// Resolve resolves the password from the credentials.
	Resolve(ctx context.Context, creds *model.Credentials) (*string, error)
}

type passwordResolverImpl struct {
	resolver Resolver
}

// NewPasswordResolver returns a PasswordResolver that uses the provided Resolver
// to resolve Secret Agent-backed values. Passing nil disables Secret Agent
// resolution (password is treated as a literal).
func NewPasswordResolver(resolver Resolver) PasswordResolver {
	return &passwordResolverImpl{resolver: resolver}
}

// Resolve resolves the password from the credentials.
// It handles reading from file or using the Secret Agent.
func (r passwordResolverImpl) Resolve(ctx context.Context, creds *model.Credentials) (*string, error) {
	if creds == nil {
		return nil, nil
	}

	// 1) Resolve Path (file)
	if creds.PasswordPath != "" {
		data, err := os.ReadFile(creds.PasswordPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read password from password-path %s: %w", creds.PasswordPath, err)
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
