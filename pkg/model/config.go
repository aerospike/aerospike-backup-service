package model

import (
	"encoding/json"
	"fmt"
)

// Config represents the service configuration file.
//
//nolint:lll
type Config struct {
	HTTPServer        *HTTPServerConfig            `yaml:"service,omitempty" json:"service,omitempty"`
	AerospikeClusters map[string]*AerospikeCluster `yaml:"aerospike-clusters,omitempty" json:"aerospike-clusters,omitempty"`
	Storage           map[string]*Storage          `yaml:"storage,omitempty" json:"storage,omitempty"`
	BackupPolicies    map[string]*BackupPolicy     `yaml:"backup-policies,omitempty" json:"backup-policies,omitempty"`
	BackupRoutines    map[string]*BackupRoutine    `yaml:"backup-routines,omitempty" json:"backup-routines,omitempty"`
}

// NewConfigWithDefaultValues returns a new Config with default values.
func NewConfigWithDefaultValues() *Config {
	return &Config{
		HTTPServer: NewHttpServerWithDefaultValues(),
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
		if err := routine.Validate(); err != nil {
			return err
		}
		if _, exists := c.AerospikeClusters[routine.SourceCluster]; !exists {
			return fmt.Errorf("BackupRoutine '%s' references a non-existent AerospikeCluster '%s'", name, routine.SourceCluster)
		}
		if _, exists := c.BackupPolicies[routine.BackupPolicy]; !exists {
			return fmt.Errorf("BackupRoutine '%s' references a non-existent BackupPolicy '%s'", name, routine.BackupPolicy)
		}
		if _, exists := c.Storage[routine.Storage]; !exists {
			return fmt.Errorf("BackupRoutine '%s' references a non-existent Storage '%s'", name, routine.Storage)
		}
	}

	for _, storage := range c.Storage {
		if err := storage.Validate(); err != nil {
			return err
		}
	}

	for _, cluster := range c.AerospikeClusters {
		if err := cluster.Validate(); err != nil {
			return err
		}
	}

	return nil
}
