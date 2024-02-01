package model

// SecretAgent represents the configuration of an Aerospike Secret Agent
// for a backup/restore operation.
// Aerospike Secret Agent acts as a proxy layer between Aerospike server and one or more
// external secrets management services, fetching secrets on behalf of the server.
//
// @Description SecretAgent represents the configuration of an Aerospike Secret Agent
// @Description for a backup/restore operation.
type SecretAgent struct {
	// Address of the Secret Agent.
	Address string `yaml:"address,omitempty" json:"address,omitempty"`
	// Port the Secret Agent is running on.
	Port string `yaml:"port,omitempty" json:"port,omitempty"`
	// Timeout in milliseconds.
	Timeout int32 `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	// The path to a trusted CA certificate file in PEM format.
	TLSCAString string `yaml:"tls-ca,omitempty" json:"tls-ca,omitempty"`
	// Indicates whether TLS is enabled.
	TLSEnabled bool `yaml:"tls-enabled,omitempty" json:"tls-enabled,omitempty"`
}
