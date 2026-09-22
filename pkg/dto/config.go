package dto

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// Config represents the service configuration file.
//
// Every entity name - the key of a cluster, storage, policy, secret agent or routine - must be
// a single path segment: a routine's name is the folder its backups live in under the storage
// root, so a name may not be "." or "..", may not contain "/" or "\\" or a NUL byte, and may
// not start with "~".
//
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

// Validate validates the configuration.
//
//nolint:gocognit
func (c *Config) Validate() error {
	for name, routine := range c.BackupRoutines {
		if err := validateEntityName("routine name", name); err != nil {
			return err
		}
		if err := routine.Validate(); err != nil {
			return fmt.Errorf("backup routine '%s' validation error: %w", name, err)
		}
	}

	for name, storage := range c.Storage {
		if err := validateEntityName("storage name", name); err != nil {
			return err
		}
		if err := storage.Validate(); err != nil {
			return fmt.Errorf("storage '%s' validation error: %w", name, err)
		}
	}

	for name, cluster := range c.AerospikeClusters {
		if err := validateEntityName("cluster name", name); err != nil {
			return err
		}
		if err := cluster.Validate(); err != nil {
			return fmt.Errorf("cluster '%s' validation error: %w", name, err)
		}
	}

	for name, policy := range c.BackupPolicies {
		if err := validateEntityName("policy name", name); err != nil {
			return err
		}
		policyOpts := ValidationDefault
		if c.backupPolicyHasSecretAgent(name) {
			policyOpts = ValidationWithSecretAgent
		}

		if err := policy.ValidateWithOpts(policyOpts); err != nil {
			return fmt.Errorf("policy '%s' validation error: %w", name, err)
		}
	}

	for name, agent := range c.SecretAgents {
		if err := validateEntityName("secret agent name", name); err != nil {
			return err
		}
		if agent == nil {
			return fmt.Errorf("secret agent '%s' validation error: secret agent is not specified", name)
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
	serviceTimezone := modelConfig.ServiceConfig.GetBackupCommonOrDefault().Timezone
	// routines must be added after storage, secret agents and policies.
	for k, v := range c.BackupRoutines {
		toModel, err := v.ToModel(backupConfig, k, serviceTimezone)
		if err != nil {
			return nil, fmt.Errorf("invalid backup routine %q: %w", k, err)
		}

		if err := modelConfig.AddRoutine(toModel); err != nil {
			return nil, err
		}
	}

	return modelConfig, nil
}

// The Add, Update, Delete and SetRoutineDisabled methods below mutate the configuration in place
// and return the names of the routines the change invalidates, so callers can reschedule them.
// A name that does not resolve yields model.ErrNotFound, a name that is already taken
// model.ErrAlreadyExists, and an entity a routine still references model.ErrInUse.

func (c *Config) AddRoutine(name string, routine *BackupRoutine) ([]string, error) {
	if _, exists := c.BackupRoutines[name]; exists {
		return nil, fmt.Errorf("add backup routine %q: %w", name, model.ErrAlreadyExists)
	}
	c.BackupRoutines[name] = routine

	return []string{name}, nil
}

func (c *Config) UpdateRoutine(name string, routine *BackupRoutine) ([]string, error) {
	if _, exists := c.BackupRoutines[name]; !exists {
		return nil, fmt.Errorf("update backup routine %q: %w", name, model.ErrNotFound)
	}
	c.BackupRoutines[name] = routine

	return []string{name}, nil
}

func (c *Config) DeleteRoutine(name string) ([]string, error) {
	if _, exists := c.BackupRoutines[name]; !exists {
		return nil, fmt.Errorf("delete backup routine %q: %w", name, model.ErrNotFound)
	}
	delete(c.BackupRoutines, name)

	return []string{name}, nil
}

func (c *Config) SetRoutineDisabled(name string, disabled bool) ([]string, error) {
	routine, exists := c.BackupRoutines[name]
	if !exists {
		return nil, fmt.Errorf("toggle disable for backup routine %q: %w", name, model.ErrNotFound)
	}
	routine.Disabled = disabled

	return []string{name}, nil
}

func (c *Config) AddStorage(name string, storage *Storage) ([]string, error) {
	if _, exists := c.Storage[name]; exists {
		return nil, fmt.Errorf("add storage %q: %w", name, model.ErrAlreadyExists)
	}
	c.Storage[name] = storage

	return nil, nil
}

func (c *Config) UpdateStorage(name string, storage *Storage) ([]string, error) {
	if _, exists := c.Storage[name]; !exists {
		return nil, fmt.Errorf("update storage %q: %w", name, model.ErrNotFound)
	}
	c.Storage[name] = storage

	return c.routinesUsingStorage(name), nil
}

func (c *Config) DeleteStorage(name string) ([]string, error) {
	if _, exists := c.Storage[name]; !exists {
		return nil, fmt.Errorf("delete storage %q: %w", name, model.ErrNotFound)
	}
	if routines := c.routinesUsingStorage(name); len(routines) > 0 {
		return nil, fmt.Errorf("delete storage %q: %w: it is used in routine %q", name, model.ErrInUse, routines[0])
	}
	delete(c.Storage, name)

	return nil, nil
}

func (c *Config) AddCluster(name string, cluster *AerospikeCluster) ([]string, error) {
	if _, exists := c.AerospikeClusters[name]; exists {
		return nil, fmt.Errorf("add Aerospike cluster %q: %w", name, model.ErrAlreadyExists)
	}
	c.AerospikeClusters[name] = cluster

	return nil, nil
}

func (c *Config) UpdateCluster(name string, cluster *AerospikeCluster) ([]string, error) {
	if _, exists := c.AerospikeClusters[name]; !exists {
		return nil, fmt.Errorf("update Aerospike cluster %q: %w", name, model.ErrNotFound)
	}
	c.AerospikeClusters[name] = cluster

	return c.routinesUsingCluster(name), nil
}

func (c *Config) DeleteCluster(name string) ([]string, error) {
	if _, exists := c.AerospikeClusters[name]; !exists {
		return nil, fmt.Errorf("delete Aerospike cluster %q: %w", name, model.ErrNotFound)
	}
	if routines := c.routinesUsingCluster(name); len(routines) > 0 {
		return nil, fmt.Errorf(
			"delete Aerospike cluster %q: %w: it is used in routine %q", name, model.ErrInUse, routines[0])
	}
	delete(c.AerospikeClusters, name)

	return nil, nil
}

func (c *Config) AddPolicy(name string, policy *BackupPolicy) ([]string, error) {
	if _, exists := c.BackupPolicies[name]; exists {
		return nil, fmt.Errorf("add backup policy %q: %w", name, model.ErrAlreadyExists)
	}
	c.BackupPolicies[name] = policy

	return nil, nil
}

func (c *Config) UpdatePolicy(name string, policy *BackupPolicy) ([]string, error) {
	if _, exists := c.BackupPolicies[name]; !exists {
		return nil, fmt.Errorf("update backup policy %q: %w", name, model.ErrNotFound)
	}
	c.BackupPolicies[name] = policy

	return c.routinesUsingPolicy(name), nil
}

func (c *Config) DeletePolicy(name string) ([]string, error) {
	if _, exists := c.BackupPolicies[name]; !exists {
		return nil, fmt.Errorf("delete backup policy %q: %w", name, model.ErrNotFound)
	}
	if routines := c.routinesUsingPolicy(name); len(routines) > 0 {
		return nil, fmt.Errorf("delete backup policy %q: %w: it is used in routine %q", name, model.ErrInUse, routines[0])
	}
	delete(c.BackupPolicies, name)

	return nil, nil
}

func (c *Config) routinesUsingStorage(name string) []string {
	return c.routineNames(func(r *BackupRoutine) bool { return r.Storage == name })
}

func (c *Config) routinesUsingCluster(name string) []string {
	return c.routineNames(func(r *BackupRoutine) bool { return r.SourceCluster == name })
}

func (c *Config) routinesUsingPolicy(name string) []string {
	return c.routineNames(func(r *BackupRoutine) bool { return r.BackupPolicy == name })
}

func (c *Config) routineNames(match func(*BackupRoutine) bool) []string {
	var names []string
	for name, routine := range c.BackupRoutines {
		if match(routine) {
			names = append(names, name)
		}
	}

	return names
}

func (c *Config) backupPolicyHasSecretAgent(policyName string) bool {
	for _, routine := range c.BackupRoutines {
		if routine != nil && routine.BackupPolicy == policyName && routine.SecretAgent != "" {
			return true
		}
	}

	return false
}
