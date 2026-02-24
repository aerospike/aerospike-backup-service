package model

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
)

// SecretAgent represents the configuration of an Aerospike Secret Agent
// for a backup/restore operation.
// Aerospike Secret Agent acts as a proxy layer between Aerospike server and one or more
// external secrets management services, fetching secrets on behalf of the server.
type SecretAgent struct {
	ClientTLS
	// Connection type: tcp, unix.
	ConnectionType string
	// Address of the Secret Agent.
	Address string
	// Port the Secret Agent is running on.
	Port *Port
	// Timeout in milliseconds.
	Timeout *int
	// Flag that shows if secret agent responses are encrypted with base64.
	IsBase64 *bool
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
		CaFile:             s.CAFile,
		TLSName:            s.Name,
		CertFile:           s.Certfile,
		KeyFile:            s.Keyfile,
		IsBase64:           s.IsBase64,
	}
}

// String returns a string representation of the SecretAgent.
func (s *SecretAgent) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("%v:%v:%v:%v:%v:%v",
		s.ConnectionType,
		s.Address,
		ptr.ValueOrZero(s.Port),
		ptr.ValueOrZero(s.Timeout),
		s.ClientTLS.String(),
		ptr.ValueOrZero(s.IsBase64))
}
