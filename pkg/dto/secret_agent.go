package dto

import (
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	saClient "github.com/aerospike/backup-go/pkg/secret-agent"
)

// SecretAgentConfig aggregates the SecretAgent configuration.
// It is intended to be embedded into DTOs that require Secret Agent configuration.
type SecretAgentConfig struct {
	// Secret Agent configuration (optional).
	// Mutually exclusive with secret-agent-name.
	SecretAgent *SecretAgent `yaml:"secret-agent,omitempty" json:"secret-agent,omitempty"`
	// Secret Agent configuration (optional). Link to one of preconfigured agents.
	// Mutually exclusive with secret-agent.
	SecretAgentName *string `yaml:"secret-agent-name,omitempty" json:"secret-agent-name,omitempty"`
}

func (c SecretAgentConfig) validate() error {
	if c.SecretAgent != nil && c.SecretAgentName != nil {
		return errors.New("secret-agent-name and secret-agent are mutually exclusive")
	}
	if err := c.SecretAgent.validate(); err != nil {
		return fmt.Errorf("secret-agent validation error: %w", err)
	}

	return nil
}

// SecretAgent represents the configuration of an Aerospike Secret Agent
// for a backup/restore operation.
// Aerospike Secret Agent acts as a proxy layer between Aerospike server and one or more
// external secrets management services, fetching secrets on behalf of the server.
//
// @Description SecretAgent represents the configuration of an Aerospike Secret Agent
// @Description for a backup/restore operation.
type SecretAgent struct {
	// Connection type: tcp, unix.
	ConnectionType string `yaml:"connection-type,omitempty" json:"connection-type,omitempty" example:"tcp"`
	// Address of the Secret Agent.
	Address string `yaml:"address,omitempty" json:"address,omitempty" example:"localhost"`
	// Port the Secret Agent is running on.
	Port *int `yaml:"port,omitempty" json:"port,omitempty" example:"8080"`
	// Timeout in milliseconds.
	Timeout *int `yaml:"timeout,omitempty" json:"timeout,omitempty" example:"5000"`
	// The path to a trusted CA certificate file in PEM format.
	TLSCAString *string `yaml:"tls-ca-file,omitempty" json:"tls-ca-file,omitempty" example:"/path/to/ca.pem"`
	// Flag that shows if secret agent responses are encrypted with base64.
	IsBase64 *bool `yaml:"is-base64,omitempty" json:"is-base64,omitempty" example:"false"`
}

func (s *SecretAgent) ToModel() *model.SecretAgent {
	if s == nil {
		return nil
	}

	return &model.SecretAgent{
		ConnectionType: s.ConnectionType,
		Address:        s.Address,
		Port:           s.Port,
		Timeout:        s.Timeout,
		TLSCAString:    s.TLSCAString,
		IsBase64:       s.IsBase64,
	}
}

func ResolveSecretAgentFromModel(s *model.SecretAgent, config *model.Config) (*string, *SecretAgent) {
	secretAgentName := findKeyByValue(config.SecretAgents, s)
	if secretAgentName != "" {
		return &secretAgentName, nil
	}

	return nil, newSecretAgentFromModel(s)
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
	s.Port = m.Port
	s.Timeout = m.Timeout
	s.TLSCAString = m.TLSCAString
	s.IsBase64 = m.IsBase64
}

// validate validates the SecretAgentConfig.
func (s *SecretAgent) validate() error {
	if s == nil {
		return nil
	}

	if s.Address == "" {
		return fmt.Errorf("address is required")
	}

	if s.Timeout != nil && *s.Timeout <= 0 {
		return fmt.Errorf("invalid timeout: %d", *s.Timeout)
	}

	if s.ConnectionType != saClient.ConnectionTypeTCP && s.ConnectionType != saClient.ConnectionTypeUDS {
		return fmt.Errorf("unsupported connection type: %s", s.ConnectionType)
	}

	if s.Port != nil && (*s.Port <= 0 || *s.Port > 65535) {
		return fmt.Errorf("'port' must be between 1 and 65535, got: %d", *s.Port)
	}

	return nil
}
