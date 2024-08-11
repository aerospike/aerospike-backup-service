package dto

// SecretAgentDTO represents the configuration of an Aerospike Secret Agent
// for a backup/restore operation.
// Aerospike Secret Agent acts as a proxy layer between Aerospike server and one or more
// external secrets management services, fetching secrets on behalf of the server.
//
// @Description SecretAgentDTO represents the configuration of an Aerospike Secret Agent
// @Description for a backup/restore operation.
type SecretAgentDTO struct {
	// Connection type: tcp, unix.
	ConnectionType *string `json:"connection-type,omitempty" example:"tcp"`
	// Address of the Secret Agent.
	Address *string `json:"address,omitempty" example:"localhost"`
	// Port the Secret Agent is running on.
	Port *int `json:"port,omitempty" example:"8080"`
	// Timeout in milliseconds.
	Timeout *int `json:"timeout,omitempty" example:"5000"`
	// The path to a trusted CA certificate file in PEM format.
	TLSCAString *string `json:"tls-ca-file,omitempty" example:"/path/to/ca.pem"`
	// Flag that shows if secret agent responses are encrypted with base64.
	IsBase64 *bool `json:"is-base64,omitempty" example:"false"`
}
