package model

import (
	"encoding/json"
	"fmt"
)

// Config represents the service configuration file.
// @Description Config represents the service configuration file.
//
//nolint:lll
type Config struct {
	HTTPServer        *HTTPServerConfig            `yaml:"service,omitempty" json:"service,omitempty"`
	AerospikeClusters map[string]*AerospikeCluster `yaml:"aerospike-clusters,omitempty" json:"aerospike-clusters,omitempty"`
	Storage           map[string]*Storage          `yaml:"storage,omitempty" json:"storage,omitempty"`
	BackupPolicies    map[string]*BackupPolicy     `yaml:"backup-policies,omitempty" json:"backup-policies,omitempty"`
	BackupRoutines    map[string]*BackupRoutine    `yaml:"backup-routines,omitempty" json:"backup-routines,omitempty"`
	SecretAgents      map[string]*SecretAgent      `yaml:"secret-agent,omitempty" json:"secret-agent,omitempty"`
}

// NewConfigWithDefaultValues returns a new Config with default values.
func NewConfigWithDefaultValues() *Config {
	return &Config{
		HTTPServer: NewHTTPServerWithDefaultValues(),
	}
}

// String satisfies the fmt.Stringer interface.
func (c Config) String() string {
	cfg, err := json.Marshal(c)
	if err != nil {
		return err.Error()
	}
	return string(cfg)
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	for name, routine := range c.BackupRoutines {
		if name == "" {
			return fmt.Errorf("empty routine name is not allowed")
		}
		if err := routine.Validate(); err != nil {
			return fmt.Errorf("backup routine '%s' validation error: %s", name, err.Error())
		}
		if _, exists := c.AerospikeClusters[routine.SourceCluster]; !exists {
			return fmt.Errorf("backup routine '%s' references a non-existent AerospikeCluster '%s'",
				name, routine.SourceCluster)
		}
		if _, exists := c.BackupPolicies[routine.BackupPolicy]; !exists {
			return fmt.Errorf("backup routine '%s' references a non-existent BackupPolicy '%s'",
				name, routine.BackupPolicy)
		}
		if _, exists := c.Storage[routine.Storage]; !exists {
			return fmt.Errorf("backup routine '%s' references a non-existent Storage '%s'",
				name, routine.Storage)
		}
		if routine.SecretAgent != nil {
			if _, exists := c.SecretAgents[*routine.SecretAgent]; !exists {
				return fmt.Errorf("backup routine '%s' references a non-existent SecretAgent '%s'",
					name, *routine.SecretAgent)
			}
		}
	}

	for name, storage := range c.Storage {
		if name == "" {
			return fmt.Errorf("empty storage name is not allowed")
		}
		if err := storage.Validate(); err != nil {
			return err
		}
	}

	for name, cluster := range c.AerospikeClusters {
		if name == "" {
			return fmt.Errorf("empty cluster name is not allowed")
		}
		if err := cluster.Validate(); err != nil {
			return err
		}
	}

	return nil
}
