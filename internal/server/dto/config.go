package dto

import (
	"fmt"
	"io"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

var _ ReadWriteDTO[model.Config] = (*Config)(nil)

// Config represents the service configuration file.
// @Description Config represents the service configuration file.
//
//nolint:lll
type Config struct {
	ServiceConfig     BackupServiceConfig          `yaml:"service,omitempty" json:"service,omitempty"`
	AerospikeClusters map[string]*AerospikeCluster `yaml:"aerospike-clusters,omitempty" json:"aerospike-clusters,omitempty"`
	Storage           map[string]*Storage          `yaml:"storage,omitempty" json:"storage,omitempty"`
	BackupPolicies    map[string]*BackupPolicy     `yaml:"backup-policies,omitempty" json:"backup-policies,omitempty"`
	BackupRoutines    map[string]*BackupRoutine    `yaml:"backup-routines,omitempty" json:"backup-routines,omitempty"`
	SecretAgents      map[string]*SecretAgent      `yaml:"secret-agent,omitempty" json:"secret-agent,omitempty"`
}

func (c *Config) Serialize(format SerializationFormat) ([]byte, error) {
	return Serialize(c, format)
}

func (c *Config) Deserialize(r io.Reader, format SerializationFormat) error {
	return Deserialize(c, r, format)
}

func (c *Config) fromModel(m *model.Config) {
	c.ServiceConfig.fromModel(&m.ServiceConfig)

	c.AerospikeClusters = make(map[string]*AerospikeCluster)
	for name, a := range m.AerospikeClusters {
		c.AerospikeClusters[name] = NewClusterFromModel(a)
	}

	c.Storage = make(map[string]*Storage)
	for name, s := range m.Storage {
		c.Storage[name] = NewStorageFromModel(s)
	}

	c.BackupPolicies = make(map[string]*BackupPolicy)
	for name, p := range m.BackupPolicies {
		c.BackupPolicies[name] = NewBackupPolicyFromModel(p)
	}

	c.BackupRoutines = make(map[string]*BackupRoutine)
	for name, r := range m.BackupRoutines {
		c.BackupRoutines[name] = NewRoutineFromModel(r, m)
	}

	c.SecretAgents = make(map[string]*SecretAgent)
	for name, s := range m.SecretAgents {
		c.SecretAgents[name] = NewSecretAgentFromModel(s)
	}
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

func NewConfigFromModel(m *model.Config) *Config {
	config := &Config{}
	config.fromModel(m)
	return config
}

// NewConfigFromReader creates a new Config object from a given reader
func NewConfigFromReader(r io.Reader, format SerializationFormat) (*Config, error) {
	c := &Config{}
	if err := c.Deserialize(r, format); err != nil {
		return nil, err
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}

	return c, nil
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	for name, routine := range c.BackupRoutines {
		if name == "" {
			return emptyFieldValidationError("routine name")
		}
		if err := routine.Validate(); err != nil {
			return fmt.Errorf("backup routine '%s' validation error: %s", name, err.Error())
		}
		if err := c.validateRoutineReferences(routine); err != nil {
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

func (c *Config) validateRoutineReferences(routine *BackupRoutine) error {
	if _, exists := c.BackupPolicies[routine.BackupPolicy]; !exists {
		return notFoundValidationError("backup policy", routine.BackupPolicy)
	}
	cluster, exists := c.AerospikeClusters[routine.SourceCluster]
	if !exists {
		return notFoundValidationError("Aerospike cluster", routine.SourceCluster)
	}
	if cluster.MaxParallelScans != nil {
		if len(routine.SetList) > *cluster.MaxParallelScans {
			return fmt.Errorf("max parallel scans must be at least the cardinality of set-list")
		}
	}
	if _, exists := c.Storage[routine.Storage]; !exists {
		return notFoundValidationError("storage", routine.Storage)
	}
	if routine.SecretAgent != nil {
		if _, exists := c.SecretAgents[*routine.SecretAgent]; !exists {
			return notFoundValidationError("secret agent", *routine.SecretAgent)
		}
	}

	return nil
}

func (c *Config) ToModel() *model.Config {
	config := c.ServiceConfig
	modelConfig := &model.Config{
		ServiceConfig:     *config.ToModel(),
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
