package model

// SecretAgent represents the configuration of an Aerospike Secret Agent
// for a backup/restore operation.
// Aerospike Secret Agent acts as a proxy layer between Aerospike server and one or more
// external secrets management services, fetching secrets on behalf of the server.
type SecretAgent struct {
	Address     string `yaml:"address,omitempty" json:"address,omitempty"`
	Port        string `yaml:"port,omitempty" json:"port,omitempty"`
	Timeout     int32  `yaml:"timeout,omitempty" json:"timeout,omitempty"` // timeout in milliseconds
	TLSCAString string `yaml:"tls-ca,omitempty" json:"tls-ca,omitempty"`
	TLSEnabled  bool   `yaml:"tls-enabled,omitempty" json:"tls-enabled,omitempty"`
}
