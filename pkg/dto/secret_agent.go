package dto

import (
	"fmt"
	"io"
	"os"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	saClient "github.com/aerospike/backup-go/pkg/secret-agent"
)

// NewSecretAgentFromReader creates a new SecretAgent from a reader.
func NewSecretAgentFromReader(r io.Reader, format decoder.SerializationFormat) (*SecretAgent, error) {
	secretAgent := &SecretAgent{}
	if err := decoder.Deserialize(secretAgent, r, format); err != nil {
		return nil, err
	}
	if err := secretAgent.validate(); err != nil {
		return nil, err
	}
	return secretAgent, nil
}

// SecretAgentConfig aggregates the SecretAgent configuration.
// It is intended to be embedded into DTOs that require Secret Agent configuration.
type SecretAgentConfig struct {
	// Secret Agent configuration (optional).
	// Mutually exclusive with 'secret-agent-name'.
	SecretAgent *SecretAgent `yaml:"secret-agent,omitempty" json:"secret-agent,omitempty"`
	// Secret Agent configuration (optional). Link to one of preconfigured agents.
	// Mutually exclusive with 'secret-agent'.
	SecretAgentName string `yaml:"secret-agent-name,omitempty" json:"secret-agent-name,omitempty" extensions:"x-nullable"`
}

func (c SecretAgentConfig) validate() error {
	if c.SecretAgent != nil && c.SecretAgentName != "" {
		return errValidationMutuallyExclusive("secret-agent-name", "secret-agent")
	}
	if err := c.SecretAgent.validate(); err != nil {
		return fmt.Errorf("secret-agent validation error: %w", err)
	}

	return nil
}

func (c *SecretAgentConfig) ToModel(config *model.Config) (*model.SecretAgent, error) {
	if c == nil { // secret agent is optional
		return nil, nil
	}

	if c.SecretAgent != nil {
		return c.SecretAgent.ToModel(), nil
	}

	if c.SecretAgentName != "" {
		agent, exists := config.BackupConfigCopy().SecretAgents[c.SecretAgentName]
		if !exists {
			return nil, fmt.Errorf("unknown secret agent %q", c.SecretAgentName)
		}
		return agent, nil
	}

	return nil, nil
}

// SecretAgent represents the configuration of an Aerospike Secret Agent
// for a backup/restore operation.
// Aerospike Secret Agent acts as a proxy layer between Aerospike server and one or more
// external secrets management services, fetching secrets on behalf of the server.
//
// @Description SecretAgent represents the configuration of an Aerospike Secret Agent.
//
//nolint:lll
type SecretAgent struct {
	// Connection type.
	//nolint:lll
	ConnectionType string `yaml:"connection-type,omitempty" json:"connection-type,omitempty" example:"tcp" validate:"required" enums:"tcp,unix"`
	// Address of the Secret Agent.
	Address string `yaml:"address" json:"address" example:"localhost" validate:"required"`
	// Port the Secret Agent is running on.
	Port *Port `yaml:"port,omitempty" json:"port,omitempty" example:"8080" extensions:"x-nullable"`
	// Timeout in milliseconds.
	Timeout *int `yaml:"timeout,omitempty" json:"timeout,omitempty" default:"1000"`
	// The path to a trusted CA certificate file in PEM format.
	TLSCAString *string `yaml:"tls-ca-file,omitempty" json:"tls-ca-file,omitempty" example:"/path/to/ca.pem" extensions:"x-nullable"`
	// Flag that shows if secret agent responses are encrypted with base64.
	IsBase64 *bool `yaml:"is-base64,omitempty" json:"is-base64,omitempty" default:"false"`
}

func (s *SecretAgent) ToModel() *model.SecretAgent {
	if s == nil {
		return nil
	}

	return &model.SecretAgent{
		ConnectionType: s.ConnectionType,
		Address:        s.Address,
		Port:           s.Port.ToModel(),
		Timeout:        s.Timeout,
		TLSCAString:    s.TLSCAString,
		IsBase64:       s.IsBase64,
	}
}

func ResolveSecretAgentFromModel(s *model.SecretAgent, config *model.BackupConfig) SecretAgentConfig {
	secretAgentName := findKeyByValue(config.SecretAgents, s)
	if secretAgentName != "" {
		return SecretAgentConfig{
			SecretAgentName: secretAgentName,
		}
	}

	return SecretAgentConfig{
		SecretAgent: newSecretAgentFromModel(s),
	}
}

func newSecretAgentFromModel(m *model.SecretAgent) *SecretAgent {
	if m == nil {
		return nil
	}

	s := &SecretAgent{}
	s.fromModel(m)
	return s
}

func (s *SecretAgent) fromModel(m *model.SecretAgent) {
	s.ConnectionType = m.ConnectionType
	s.Address = m.Address
	s.Port = NewPortFromModel(m.Port)
	s.Timeout = m.Timeout
	s.TLSCAString = m.TLSCAString
	s.IsBase64 = m.IsBase64
}

// validate validates the SecretAgent.
func (s *SecretAgent) validate() error {
	if s == nil {
		return nil
	}

	if s.Address == "" {
		return errValidationEmptyField("address")
	}

	if s.Timeout != nil && *s.Timeout < 0 {
		return errValidationNegative("timeout", *s.Timeout)
	}

	if s.TLSCAString != nil {
		_, err := os.Stat(*s.TLSCAString)
		if err != nil {
			return errValidationNotFound("tls-ca-file", *s.TLSCAString)
		}
	}

	if s.ConnectionType == "" {
		return errValidationEmptyField("connection-type")
	}

	if s.ConnectionType != saClient.ConnectionTypeTCP && s.ConnectionType != saClient.ConnectionTypeUDS {
		return errValidationInvalidValue("connection-type", s.ConnectionType,
			[]string{saClient.ConnectionTypeTCP, saClient.ConnectionTypeUDS})
	}

	if err := s.Port.Validate(); err != nil {
		return err
	}

	return nil
}
