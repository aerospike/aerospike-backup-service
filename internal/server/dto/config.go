package dto

import (
	"fmt"
	"io"

	"github.com/aerospike/aerospike-backup-service/pkg/model"
)

var _ ReadWriteDTO[model.Config] = (*Config)(nil)

// Config represents the service configuration file.
// @Description Config represents the service configuration file.
//
//nolint:lll
type Config struct {
	ServiceConfig     *BackupServiceConfig        `yaml:"service,omitempty" json:"service,omitempty"`
	AerospikeClusters map[string]AerospikeCluster `yaml:"aerospike-clusters,omitempty" json:"aerospike-clusters,omitempty"`
	Storage           map[string]Storage          `yaml:"storage,omitempty" json:"storage,omitempty"`
	BackupPolicies    map[string]BackupPolicy     `yaml:"backup-policies,omitempty" json:"backup-policies,omitempty"`
	BackupRoutines    map[string]BackupRoutine    `yaml:"backup-routines,omitempty" json:"backup-routines,omitempty"`
	SecretAgents      map[string]SecretAgent      `yaml:"secret-agent,omitempty" json:"secret-agent,omitempty"`
}

func (c *Config) Serialize(format SerializationFormat) ([]byte, error) {
	return Serialize(c, format)
}

func (c *Config) Deserialize(r io.Reader, format SerializationFormat) error {
	return Deserialize(c, r, format)
}

func (c *Config) fromModel(m *model.Config) {
	//TODO implement me
	panic("implement me")
}

// NewConfigWithDefaultValues returns a new Config with default values.
func NewConfigWithDefaultValues() *Config {
	return &Config{
		ServiceConfig:     NewBackupServiceConfigWithDefaultValues(),
		Storage:           map[string]Storage{},
		BackupRoutines:    map[string]BackupRoutine{},
		BackupPolicies:    map[string]BackupPolicy{},
		AerospikeClusters: map[string]AerospikeCluster{},
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	for name, routine := range c.BackupRoutines {
		if name == "" {
			return emptyFieldValidationError("routine name")
		}
		if err := routine.Validate(c); err != nil {
			return fmt.Errorf("backup routine '%s' validation error: %s", name, err.Error())
		}
	}

	for name, storage := range c.Storage {
		if name == "" {
			return emptyFieldValidationError("storage name")
		}
		if err := storage.Validate(); err != nil {
			return fmt.Errorf("storage '%s' validation error: %s", name, err.Error())
		}
	}

	for name, cluster := range c.AerospikeClusters {
		if name == "" {
			return emptyFieldValidationError("cluster name")
		}
		if err := cluster.Validate(); err != nil {
			return fmt.Errorf("cluster '%s' validation error: %s", name, err.Error())
		}
	}

	for name, policy := range c.BackupPolicies {
		if name == "" {
			return emptyFieldValidationError("policy name")
		}
		if err := policy.Validate(); err != nil {
			return err
		}
	}

	if err := c.ServiceConfig.HTTPServer.Validate(); err != nil {
		return err
	}

	if err := c.ServiceConfig.Logger.Validate(); err != nil {
		return err
	}

	return nil
}

func (c *Config) ToModel() *model.Config {
	modelConfig := &model.Config{
		ServiceConfig:     c.ServiceConfig.ToModel(),
		AerospikeClusters: make(map[string]*model.AerospikeCluster),
		Storage:           make(map[string]*model.Storage),
		BackupPolicies:    make(map[string]*model.BackupPolicy),
		BackupRoutines:    make(map[string]*model.BackupRoutine),
		SecretAgents:      make(map[string]*model.SecretAgent),
	}

	for k, v := range c.AerospikeClusters {
		modelConfig.AerospikeClusters[k] = v.ToModel()
	}

	for k, v := range c.Storage {
		modelConfig.Storage[k] = v.ToModel()
	}

	for k, v := range c.BackupPolicies {
		modelConfig.BackupPolicies[k] = v.ToModel()
	}

	for k, v := range c.SecretAgents {
		modelConfig.SecretAgents[k] = v.ToModel()
	}

	for k, v := range c.BackupRoutines {
		modelConfig.BackupRoutines[k] = v.ToModel(modelConfig)
	}

	return modelConfig
}
func emptyFieldValidationError(field string) error {
	return fmt.Errorf("empty %s is not allowed", field)
}

func notFoundValidationError(field string, value string) error {
	return fmt.Errorf("%s '%s' not found", field, value)
}
