//go:build integration

package integration

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// Docker image from https://aerospike.com/docs/database/tools/secret-agent/install
	secretAgentImage = "aerospike/aerospike-secret-agent:1.4.0" //nolint:gosec // image name, not a credential
	secretResource   = "backup"
	secretKeyName    = "key"
	agentTCPPort     = "3005/tcp"
	secretsFilePath  = "/secretagent/secrets.json"
	agentConfigPath  = "/secretagent/config.yaml"
)

// secretRef is the reference to the one secret startSecretAgent holds. For an agent
// started with startSecretAgentWithKeys, use secretRefKey.
func secretRef() string {
	return secretRefKey(secretKeyName)
}

// secretRefKey is the secrets:<resource>:<key> reference ABS resolves to the value
// startSecretAgentWithKeys stored under key.
func secretRefKey(key string) string {
	return fmt.Sprintf("secrets:%s:%s", secretResource, key)
}

// startSecretAgent starts Aerospike Secret Agent holding a single secret, addressed
// by secretRef.
func (s *Suite) startSecretAgent(secret string) *dto.SecretAgent {
	return s.startSecretAgentWithKeys(map[string]string{secretKeyName: secret})
}

// startSecretAgentWithKeys starts Aerospike Secret Agent with the file backend
// (https://github.com/aerospike/aerospike-secret-agent/blob/main/docs/file.md),
// holding one secret per map entry, each addressed by secretRefKey(key). It returns
// the config ABS needs to reach it.
func (s *Suite) startSecretAgentWithKeys(secrets map[string]string) *dto.SecretAgent {
	ctx := s.T().Context()

	agent, err := testcontainers.Run(ctx, secretAgentImage,
		testcontainers.WithCmd("--config-file", agentConfigPath),
		testcontainers.WithExposedPorts(agentTCPPort),
		testcontainers.WithFiles(
			testcontainers.ContainerFile{
				Reader:            strings.NewReader(s.secretAgentConfigYAML()),
				ContainerFilePath: agentConfigPath,
				FileMode:          0o644,
			},
			testcontainers.ContainerFile{
				Reader:            strings.NewReader(s.secretsJSON(secrets)),
				ContainerFilePath: secretsFilePath,
				FileMode:          0o600,
			},
		),
		testcontainers.WithWaitStrategy(wait.ForListeningPort(agentTCPPort)),
	)
	s.Require().NoError(err)
	s.terminateOnCleanup(agent, "secret-agent")

	host, err := agent.Host(ctx)
	s.Require().NoError(err)

	mapped, err := agent.MappedPort(ctx, agentTCPPort)
	s.Require().NoError(err)

	return &dto.SecretAgent{
		ConnectionType: "tcp",
		Address:        host,
		Port:           ptr.Of(dto.Port(mapped.Num())),
		Timeout:        ptr.Of(5000),
		// The file backend stores values base64-encoded and returns them as stored,
		// so the client has to decode. Without this the caller gets the encoded
		// string: harmless for an encryption key that is only compared to itself,
		// but a cluster password fails with INVALID_CREDENTIAL.
		IsBase64: ptr.Of(true),
	}
}

func (s *Suite) secretAgentConfigYAML() string {
	data, err := decoder.Marshal(map[string]any{
		"service": map[string]any{
			"tcp": map[string]any{
				"endpoint": "0.0.0.0:" + strings.TrimSuffix(agentTCPPort, "/tcp"),
			},
		},
		"secret-manager": map[string]any{
			"file": map[string]any{
				"resources": map[string]string{
					secretResource: secretsFilePath,
				},
			},
		},
		"log": map[string]any{
			"level": "info",
		},
	}, decoder.YAML, false)
	s.Require().NoError(err)

	return string(data)
}

// secretsJSON is the file backend's secrets file. Its values must be base64-encoded.
func (s *Suite) secretsJSON(secrets map[string]string) string {
	encoded := make(map[string]string, len(secrets))
	for key, value := range secrets {
		encoded[key] = base64.StdEncoding.EncodeToString([]byte(value))
	}

	data, err := decoder.Marshal(encoded, decoder.JSON, false)
	s.Require().NoError(err)

	return string(data)
}
