package secrets

import (
	"errors"
	"log/slog"
	"os"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// PasswordResolver resolves password from either a file, a literal value,
// or via a Secret Agent.
type PasswordResolver interface {
	// Resolve resolves the password from the credentials.
	Resolve(creds *model.Credentials) (*string, error)
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
func (r passwordResolverImpl) Resolve(creds *model.Credentials) (*string, error) {
	if creds == nil {
		return nil, nil
	}

	// 1) Resolve Path (file)
	if creds.PasswordPath != nil {
		data, err := os.ReadFile(*creds.PasswordPath)
		if err != nil {
			slog.Error("Failed to read password",
				slog.String("path", *creds.PasswordPath),
				attr.Error(err))
			return nil, err
		}
		slog.Debug("Successfully read password", slog.String("path", *creds.PasswordPath))

		password := string(data)

		return &password, nil
	}

	// 2) Resolve (literal or secret-agent reference)
	if creds.Password != nil {
		password, err := r.resolver.Resolve(creds.SecretAgent, *creds.Password)
		if err != nil {
			slog.Warn("Failed to read password from secret agent", attr.Error(err))
			return nil, err
		}

		return &password, nil
	}

	return nil, errors.New("no valid authentication method configured")
}
