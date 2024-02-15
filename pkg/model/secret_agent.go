package model

type SecretAgent struct {
	// Address of the Secret Agent.
	Address string `yaml:"address,omitempty" json:"address,omitempty" example:"localhost"`
	// Port the Secret Agent is running on.
	Port string `yaml:"port,omitempty" json:"port,omitempty" example:"8080"`
	// Timeout in milliseconds.
	Timeout int32 `yaml:"timeout,omitempty" json:"timeout,omitempty" example:"5000"`
	// The path to a trusted CA certificate file in PEM format.
	TLSCAString string `yaml:"tls-ca,omitempty" json:"tls-ca,omitempty" example:"/path/to/ca.pem"`
	// Indicates whether TLS is enabled.
	TLSEnabled bool `yaml:"tls-enabled,omitempty" json:"tls-enabled,omitempty"`
}
