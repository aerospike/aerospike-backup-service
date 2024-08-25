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
	ConnectionType *string
	// Address of the Secret Agent.
	Address *string
	// Port the Secret Agent is running on.
	Port *int
	// Timeout in milliseconds.
	Timeout *int
	// The path to a trusted CA certificate file in PEM format.
	TLSCAString *string
	// Flag that shows if secret agent responses are encrypted with base64.
	IsBase64 *bool
}
