package model

// Config represents the service configuration file.
// @Description Config represents the service configuration file.
//
//nolint:lll
type Config struct {
	ServiceConfig     *BackupServiceConfig         `yaml:"service,omitempty" json:"service,omitempty"`
	AerospikeClusters map[string]*AerospikeCluster `yaml:"aerospike-clusters,omitempty" json:"aerospike-clusters,omitempty"`
	Storage           map[string]*Storage          `yaml:"storage,omitempty" json:"storage,omitempty"`
	BackupPolicies    map[string]*BackupPolicy     `yaml:"backup-policies,omitempty" json:"backup-policies,omitempty"`
	BackupRoutines    map[string]*BackupRoutine    `yaml:"backup-routines,omitempty" json:"backup-routines,omitempty"`
	SecretAgents      map[string]*SecretAgent      `yaml:"secret-agent,omitempty" json:"secret-agent,omitempty"`
}

// NewConfigWithDefaultValues returns a new Config with default values.
func NewConfigWithDefaultValues() *Config {
	return &Config{
		ServiceConfig:     NewBackupServiceConfigWithDefaultValues(),
		Storage:           map[string]*Storage{},
		BackupRoutines:    map[string]*BackupRoutine{},
		BackupPolicies:    map[string]*BackupPolicy{},
		AerospikeClusters: map[string]*AerospikeCluster{},
	}
}
