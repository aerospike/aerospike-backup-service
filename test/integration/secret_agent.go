//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// Docker image from https://aerospike.com/docs/database/tools/secret-agent/install
	secretAgentImage   = "aerospike/aerospike-secret-agent:1.4.0" //nolint:gosec // image name, not a credential
	secretResource     = "backup"
	secretKeyName      = "key"
	secretAgentTCPPort = "3005"
	secretsFilePath    = "/secretagent/secrets.json"
)

func secretRef() string {
	return fmt.Sprintf("secrets:%s:%s", secretResource, secretKeyName)
}

// startSecretAgent starts Aerospike Secret Agent with the file backend described in
// https://github.com/aerospike/aerospike-secret-agent/blob/main/docs/file.md
func (s *Suite) startSecretAgent(secret string) *dto.SecretAgent {
	ctx := s.T().Context()

	agent, err := testcontainers.Run(ctx, secretAgentImage,
		testcontainers.WithCmd("--config-file", "/secretagent/config.yaml"),
		testcontainers.WithExposedPorts(secretAgentTCPPort+"/tcp"),
		testcontainers.WithFiles(
			testcontainers.ContainerFile{
				Reader:            strings.NewReader(s.secretAgentConfigYAML()),
				ContainerFilePath: "/secretagent/config.yaml",
				FileMode:          0o644,
			},
			testcontainers.ContainerFile{
				Reader:            strings.NewReader(secretsJSON(secret)),
				ContainerFilePath: secretsFilePath,
				FileMode:          0o600,
			},
		),
		testcontainers.WithWaitStrategy(wait.ForListeningPort(secretAgentTCPPort+"/tcp")),
	)
	s.Require().NoError(err)
	s.cleanupContainer(agent)

	host, err := agent.Host(ctx)
	s.Require().NoError(err)

	mapped, err := agent.MappedPort(ctx, secretAgentTCPPort+"/tcp")
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
				"endpoint": "0.0.0.0:" + secretAgentTCPPort,
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

func secretsJSON(secret string) string {
	// File-backend values must be base64-encoded strings.
	return fmt.Sprintf(`{%q:%q}`, secretKeyName, base64.StdEncoding.EncodeToString([]byte(secret)))
}

func (s *Suite) cleanupContainer(c testcontainers.Container) {
	t := s.T()
	ctx := context.Background()

	t.Cleanup(func() {
		if t.Failed() {
			logs, err := c.Logs(ctx)
			if err != nil {
				t.Logf("secret-agent logs unavailable: %v", err)
			} else {
				b, _ := io.ReadAll(logs)
				_ = logs.Close()
				t.Logf("secret-agent logs:\n%s", b)
			}
		}

		if err := c.Terminate(ctx); err != nil {
			t.Logf("failed to terminate secret-agent container: %v", err)
		}
	})
}

func generateEncryptionPEM() (string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", err
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
}
