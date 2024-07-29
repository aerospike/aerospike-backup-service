package model

// SecretAgent represents the configuration of an Aerospike Secret Agent
// for a backup/restore operation.
// Aerospike Secret Agent acts as a proxy layer between Aerospike server and one or more
// external secrets management services, fetching secrets on behalf of the server.
//
// @Description SecretAgent represents the configuration of an Aerospike Secret Agent
// @Description for a backup/restore operation.
type SecretAgent struct {
	// Connection type: tcp, unix.
	ConnectionType string `yaml:"sa-connection-type,omitempty" json:"sa-connection-type,omitempty"`
	// Address of the Secret Agent.
	Address string `yaml:"address,omitempty" json:"address,omitempty" example:"localhost"`
	// Port the Secret Agent is running on.
	Port int `yaml:"port,omitempty" json:"port,omitempty" example:"8080"`
	// Timeout in milliseconds.
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty" example:"5000"`
	// The path to a trusted CA certificate file in PEM format.
	TLSCAString string `yaml:"tls-ca,omitempty" json:"tls-ca,omitempty" example:"/path/to/ca.pem"`
	// Flag that shows if secret agent responses are encrypted with base64.
	IsBase64 bool `yaml:"sa-is-base64,omitempty" json:"sa-is-base64,omitempty"`
}
