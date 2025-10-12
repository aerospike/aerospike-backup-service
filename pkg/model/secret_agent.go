package model

import (
	"context"
	"fmt"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
)

// SecretAgent represents the configuration of an Aerospike Secret Agent
// for a backup/restore operation.
// Aerospike Secret Agent acts as a proxy layer between Aerospike server and one or more
// external secrets management services, fetching secrets on behalf of the server.
type SecretAgent struct {
	once sync.Once
	// Connection type: tcp, unix.
	ConnectionType string
	// Address of the Secret Agent.
	Address string
	// Port the Secret Agent is running on.
	Port *Port
	// Timeout in milliseconds.
	Timeout *int
	// The path to a trusted CA certificate file in PEM format.
	TLSCAString *string
	// Flag that shows if secret agent responses are encrypted with base64.
	IsBase64 *bool
	cache    *collections.LoadingCache[string, string]
}

func (s *SecretAgent) ToSecretAgentConfig() *backup.SecretAgentConfig {
	if s == nil {
		return nil
	}

	return &backup.SecretAgentConfig{
		ConnectionType:     &s.ConnectionType,
		Address:            &s.Address,
		Port:               (*int)(s.Port),
		TimeoutMillisecond: s.Timeout,
		CaFile:             s.TLSCAString,
		IsBase64:           s.IsBase64,
	}
}

// Read reads the secret at the given path using the Secret Agent.
// If no secret agent is configured, it returns the original path.
// If given path is not SA path (not starts with "secrets:"), it returns the original path.
func (s *SecretAgent) Read(path string) (string, error) {
	if s == nil { // If no secret agent configured, return original path.
		return path, nil
	}

	// Initialize cache only once.
	s.once.Do(func() {
		agentConfig := s.ToSecretAgentConfig()
		readFromSecretAgentfunc := func(_ context.Context, key string) (string, error) {
			secret, err := backup.ParseSecret(agentConfig, key)
			if err != nil {
				return "", fmt.Errorf("failed to read secret %q from %s: %w", key, s.Address, err)
			}

			return secret, nil
		}

		s.cache = collections.NewLoadingCache(context.Background(), readFromSecretAgentfunc)
	})

	return s.cache.Get(path)
}

// String returns a string representation of the SecretAgent.
func (s *SecretAgent) String() string {
	if s == nil {
		return nilString
	}
	return fmt.Sprintf("%v:%v:%v:%v:%v:%v",
		s.ConnectionType,
		s.Address,
		ptr.ValueOrZero(s.Port),
		ptr.ValueOrZero(s.Timeout),
		ptr.ValueOrZero(s.TLSCAString),
		ptr.ValueOrZero(s.IsBase64))
}
