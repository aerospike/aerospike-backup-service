package dto

import "github.com/aerospike/aerospike-backup-service/v2/pkg/model"

// SecretAgent represents the configuration of an Aerospike Secret Agent
// for a backup/restore operation.
// Aerospike Secret Agent acts as a proxy layer between Aerospike server and one or more
// external secrets management services, fetching secrets on behalf of the server.
//
// @Description SecretAgent represents the configuration of an Aerospike Secret Agent
// @Description for a backup/restore operation.
type SecretAgent struct {
	// Connection type: tcp, unix.
	ConnectionType *string `yaml:"connection-type,omitempty" json:"connection-type,omitempty" example:"tcp"`
	// Address of the Secret Agent.
	Address *string `yaml:"address,omitempty" json:"address,omitempty" example:"localhost"`
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

func NewSecretAgentFromModel(m *model.SecretAgent) *SecretAgent {
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
