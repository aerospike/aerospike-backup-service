package model

// SecretAgent represents the configuration of an Aerospike Secret Agent.
type SecretAgent struct {
	Address         string `yaml:"address,omitempty" json:"address,omitempty"`
	Port            string `yaml:"port,omitempty" json:"port,omitempty"`
	Timeout         int32  `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	SecretAgentFile string `yaml:"sa-file,omitempty" json:"sa-file,omitempty"`
	TLSEnabled      bool   `yaml:"tls-enabled,omitempty" json:"tls-enabled,omitempty"`
}
