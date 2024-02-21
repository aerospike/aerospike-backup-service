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
		if err := routine.Validate(c); err != nil {
			return fmt.Errorf("backup routine '%s' validation error: %s", name, err.Error())
		}
	}

	for name, storage := range c.Storage {
		if name == "" {
			return fmt.Errorf("empty storage name is not allowed")
		}
		if err := storage.Validate(); err != nil {
			return fmt.Errorf("storage '%s' validation error: %s", name, err.Error())
		}
	}

	for name, cluster := range c.AerospikeClusters {
		if name == "" {
			return fmt.Errorf("empty cluster name is not allowed")
		}
		if err := cluster.Validate(); err != nil {
			return fmt.Errorf("cluster '%s' validation error: %s", name, err.Error())
		}
	}

	for name, policy := range c.BackupPolicies {
		if name == "" {
			return fmt.Errorf("empty policy name is not allowed")
		}
		if err := policy.Validate(); err != nil {
			return err
		}
	}

	if err := c.ServiceConfig.HTTPServer.Validate(); err != nil {
		return err
	}

	if err := c.ServiceConfig.Logger.Validate(); err != nil { //nolint:revive
		return err
	}

	return nil
}
