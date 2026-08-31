package dto

import (
	"fmt"
	"io"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// Config represents the service configuration file.
// @Description Config represents the service configuration file.
//
//nolint:lll
type Config struct {
	// ServiceConfig contains general service settings.
	ServiceConfig ServiceConfig `yaml:"service,omitempty" json:"service,omitzero"`
	// AerospikeClusters is a map of Aerospike clusters that can be used by backup routines.
	AerospikeClusters map[string]*AerospikeCluster `yaml:"aerospike-clusters,omitempty" json:"aerospike-clusters,omitempty"`
	// Storage is a map of storages that can be used by backup routines.
	Storage map[string]*Storage `yaml:"storage,omitempty" json:"storage,omitempty"`
	// BackupPolicies is a map of backup policies that can be used by backup routines.
	BackupPolicies map[string]*BackupPolicy `yaml:"backup-policies,omitempty" json:"backup-policies,omitempty"`
	// SecretAgents is a map of secret agents used by backup routines (for encryption keys), clusters (for credentials), and storage (for authentication).
	SecretAgents map[string]*SecretAgent `yaml:"secret-agents,omitempty" json:"secret-agents,omitempty"`
	// BackupRoutines is a map of backup routines.
	BackupRoutines map[string]*BackupRoutine `yaml:"backup-routines,omitempty" json:"backup-routines,omitempty"`
}

func NewConfigFromModel(m *model.Config) *Config {
	if m == nil {
		return nil
	}

	config := &Config{}
	config.fromModel(m)
	return config
}

func (c *Config) fromModel(m *model.Config) {
	c.ServiceConfig.fromModel(&m.ServiceConfig)
	backupConfig := m.BackupConfigCopy()
	if c.ServiceConfig.ServerHTTPS != nil {
		c.ServiceConfig.ServerHTTPS.SecretAgentConfig = ResolveSecretAgentFromModel(
			m.ServiceConfig.ServerHTTPS.SecretAgent, backupConfig,
		)
	}

	c.AerospikeClusters = make(map[string]*AerospikeCluster)
	for name, a := range backupConfig.AerospikeClusters {
		c.AerospikeClusters[name] = NewClusterFromModel(a, backupConfig)
	}

	c.Storage = make(map[string]*Storage)
	for name, s := range backupConfig.Storage {
		c.Storage[name] = NewStorageFromModel(s, backupConfig)
	}

	c.BackupPolicies = make(map[string]*BackupPolicy)
	for name, p := range backupConfig.BackupPolicies {
		c.BackupPolicies[name] = NewBackupPolicyFromModel(p)
	}

	c.BackupRoutines = make(map[string]*BackupRoutine)
	for name, r := range backupConfig.BackupRoutines {
		c.BackupRoutines[name] = NewRoutineFromModel(r, m)
	}

	c.SecretAgents = make(map[string]*SecretAgent)
	for name, s := range backupConfig.SecretAgents {
		c.SecretAgents[name] = newSecretAgentFromModel(s)
	}
}

// NewConfigFromReader creates a new Config object from a given reader.
func NewConfigFromReader(r io.Reader, format decoder.SerializationFormat) (*Config, error) {
	c := &Config{}
	if err := decoder.Deserialize(c, r, format); err != nil {
		return nil, err
	}

	return c, nil
}

// Validate validates the configuration.
//
//nolint:gocognit
func (c *Config) Validate() error {
	for name, routine := range c.BackupRoutines {
		if name == "" {
			return errValidationEmptyField("routine name")
		}
		if err := routine.Validate(); err != nil {
			return fmt.Errorf("backup routine '%s' validation error: %w", name, err)
		}
	}

	for name, storage := range c.Storage {
		if name == "" {
			return errValidationEmptyField("storage name")
		}
		if err := storage.Validate(); err != nil {
			return fmt.Errorf("storage '%s' validation error: %w", name, err)
		}
	}

	for name, cluster := range c.AerospikeClusters {
		if name == "" {
			return errValidationEmptyField("cluster name")
		}
		if err := cluster.Validate(); err != nil {
			return fmt.Errorf("cluster '%s' validation error: %w", name, err)
		}
	}

	for name, policy := range c.BackupPolicies {
		if name == "" {
			return errValidationEmptyField("policy name")
		}
		policyOpts := ValidationDefault
		if c.backupPolicyHasSecretAgent(name) {
			policyOpts = ValidationWithSecretAgent
		}

		if err := policy.Validate(policyOpts); err != nil {
			return fmt.Errorf("policy '%s' validation error: %w", name, err)
		}
	}

	for name, agent := range c.SecretAgents {
		if name == "" {
			return errValidationEmptyField("secret agent name")
		}
		if err := agent.validate(); err != nil {
			return fmt.Errorf("secret agent '%s' validation error: %w", name, err)
		}
	}

	if err := c.ServiceConfig.Validate(); err != nil {
		return fmt.Errorf("service validation error: %w", err)
	}

	return nil
}

func (c *Config) ToModel() (*model.Config, error) {
	config := c.ServiceConfig
	modelConfig := model.NewConfig()
	modelConfig.ServiceConfig = *config.ToModel()

	for k, v := range c.SecretAgents {
		if err := modelConfig.AddSecretAgent(k, v.ToModel()); err != nil {
			return nil, err
		}
	}
	if c.ServiceConfig.ServerHTTPS != nil {
		// Resolve the HTTPS Secret Agent after the top-level agents have been added.
		agent, err := c.ServiceConfig.ServerHTTPS.SecretAgentConfig.ToModel(modelConfig)
		if err != nil {
			return nil, fmt.Errorf("invalid HTTPS server secret agent: %w", err)
		}
		modelConfig.ServiceConfig.ServerHTTPS.SecretAgent = agent
	}

	// storage must be added after secret agents.
	for k, v := range c.Storage {
		toModel, err := v.ToModel(modelConfig)
		if err != nil {
			return nil, fmt.Errorf("invalid storage %q: %w", k, err)
		}
		if err := modelConfig.AddStorage(k, toModel); err != nil {
			return nil, err
		}
	}

	for k, v := range c.BackupPolicies {
		if err := modelConfig.AddPolicy(k, v.ToModel()); err != nil {
			return nil, err
		}
	}

	// clusters must be added after secret agents.
	for k, v := range c.AerospikeClusters {
		toModel, err := v.ToModel(modelConfig)
		if err != nil {
			return nil, fmt.Errorf("invalid cluster %q: %w", k, err)
		}

		if err := modelConfig.AddCluster(k, toModel); err != nil {
			return nil, err
		}
	}

	backupConfig := modelConfig.BackupConfigCopy()
	defaultTimezone := modelConfig.ServiceConfig.GetBackupCommonOrDefault().GetTimezoneOrDefault()
	// routines must be added after storage, secret agents and policies.
	for k, v := range c.BackupRoutines {
		toModel, err := v.ToModel(backupConfig, k, defaultTimezone)
		if err != nil {
			return nil, fmt.Errorf("invalid backup routine %q: %w", k, err)
		}

		if err := modelConfig.AddRoutine(toModel); err != nil {
			return nil, err
		}
	}

	return modelConfig, nil
}

func (c *Config) backupPolicyHasSecretAgent(policyName string) bool {
	for _, routine := range c.BackupRoutines {
		if routine != nil && routine.BackupPolicy == policyName && routine.SecretAgent != "" {
			return true
		}
	}

	return false
}
