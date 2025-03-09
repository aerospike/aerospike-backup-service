package model

import (
	"fmt"
	"maps"
	"sync"
)

// Config represents the service configuration.
type Config struct {
	mu            sync.RWMutex
	backupConfig  BackupConfig
	ServiceConfig BackupServiceConfig
}

type BackupConfig struct {
	AerospikeClusters   map[string]*AerospikeCluster
	Storage             map[string]Storage // Storage is an interface
	BackupPolicies      map[string]*BackupPolicy
	BackupRoutines      map[string]*BackupRoutine
	SecretAgents        map[string]*SecretAgent
	invalidatedRoutines []string
}

func (bc *BackupConfig) copy() *BackupConfig {
	if bc == nil {
		return nil
	}

	newConfig := &BackupConfig{
		AerospikeClusters: make(map[string]*AerospikeCluster, len(bc.AerospikeClusters)),
		Storage:           make(map[string]Storage, len(bc.Storage)),
		BackupPolicies:    make(map[string]*BackupPolicy, len(bc.BackupPolicies)),
		BackupRoutines:    make(map[string]*BackupRoutine, len(bc.BackupRoutines)),
		SecretAgents:      make(map[string]*SecretAgent, len(bc.SecretAgents)),
	}

	maps.Copy(newConfig.AerospikeClusters, bc.AerospikeClusters)
	maps.Copy(newConfig.Storage, bc.Storage)
	maps.Copy(newConfig.BackupRoutines, bc.BackupRoutines)
	maps.Copy(newConfig.BackupPolicies, bc.BackupPolicies)
	maps.Copy(newConfig.SecretAgents, bc.SecretAgents)

	return newConfig
}

func newBackupConfig() *BackupConfig {
	return &BackupConfig{
		AerospikeClusters: make(map[string]*AerospikeCluster),
		Storage:           make(map[string]Storage),
		BackupPolicies:    make(map[string]*BackupPolicy),
		BackupRoutines:    make(map[string]*BackupRoutine),
		SecretAgents:      make(map[string]*SecretAgent),
	}
}

func NewConfig() *Config {
	return &Config{
		backupConfig: *newBackupConfig(),
	}
}

var (
	ErrAlreadyExists = fmt.Errorf("item already exists")
	ErrNotFound      = fmt.Errorf("item not found")
	ErrInUse         = fmt.Errorf("item is in use")
)

func (c *Config) BackupConfigCopy() *BackupConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.backupConfig.copy()
}

func (c *Config) AddStorage(name string, s Storage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.Storage[name]; exists {
		return fmt.Errorf("add storage %q: %w", name, ErrAlreadyExists)
	}
	c.backupConfig.Storage[name] = s

	return nil
}

func (c *Config) DeleteStorage(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	s, exists := c.backupConfig.Storage[name]
	if !exists {
		return fmt.Errorf("delete storage %q: %w", name, ErrNotFound)
	}
	if routine := c.routineUsesStorage(s); routine != "" {
		return fmt.Errorf("delete storage %q: %w: it is used in routine %q", name, ErrInUse, routine)
	}
	delete(c.backupConfig.Storage, name)

	return nil
}

func (c *Config) routineUsesStorage(s Storage) string {
	for name, r := range c.backupConfig.BackupRoutines {
		if r.Storage == s {
			return name
		}
	}
	return ""
}

func (c *Config) UpdateStorage(name string, s Storage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.Storage[name]; !exists {
		return fmt.Errorf("update storage %q: %w", name, ErrNotFound)
	}

	oldStorage := c.backupConfig.Storage[name]
	for routineName, r := range c.backupConfig.BackupRoutines {
		if r.Storage == oldStorage {
			r.Storage = s
			c.backupConfig.invalidatedRoutines = append(c.backupConfig.invalidatedRoutines, routineName)
		}
	}

	c.backupConfig.Storage[name] = s

	return nil
}

func (c *Config) AddPolicy(name string, p *BackupPolicy) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.BackupPolicies[name]; exists {
		return fmt.Errorf("add backup policy %q: %w", name, ErrAlreadyExists)
	}
	c.backupConfig.BackupPolicies[name] = p
	return nil
}

func (c *Config) DeletePolicy(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	p, exists := c.backupConfig.BackupPolicies[name]
	if !exists {
		return fmt.Errorf("delete backup policy %q: %w", name, ErrNotFound)
	}
	if routine := c.routineUsesPolicy(p); routine != "" {
		return fmt.Errorf("delete backup policy %q: %w: it is used in routine %q", name, ErrInUse, routine)
	}
	delete(c.backupConfig.BackupPolicies, name)

	return nil
}

func (c *Config) routineUsesPolicy(p *BackupPolicy) string {
	for name, r := range c.backupConfig.BackupRoutines {
		if r.BackupPolicy == p {
			return name
		}
	}
	return ""
}

func (c *Config) UpdatePolicy(name string, p *BackupPolicy) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.BackupPolicies[name]; !exists {
		return fmt.Errorf("update backup policy %q: %w", name, ErrNotFound)
	}

	oldPolicy := c.backupConfig.BackupPolicies[name]
	for _, r := range c.backupConfig.BackupRoutines {
		if r.BackupPolicy == oldPolicy {
			r.BackupPolicy = p
		}
	}
	c.backupConfig.BackupPolicies[name] = p

	return nil
}

func (c *Config) Routines() map[string]*BackupRoutine {
	c.mu.RLock()
	defer c.mu.RUnlock()

	routines := make(map[string]*BackupRoutine, len(c.backupConfig.BackupRoutines))
	maps.Copy(routines, c.backupConfig.BackupRoutines)

	return routines
}

func (c *Config) AddRoutine(name string, r *BackupRoutine) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.BackupRoutines[name]; exists {
		return fmt.Errorf("add backup routine %q: %w", name, ErrAlreadyExists)
	}
	c.backupConfig.BackupRoutines[name] = r
	c.backupConfig.invalidatedRoutines = append(c.backupConfig.invalidatedRoutines, name)

	return nil
}

func (c *Config) DeleteRoutine(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.BackupRoutines[name]; !exists {
		return fmt.Errorf("delete backup routine %q: %w", name, ErrNotFound)
	}
	delete(c.backupConfig.BackupRoutines, name)

	return nil
}

func (c *Config) UpdateRoutine(name string, r *BackupRoutine) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.BackupRoutines[name]; !exists {
		return fmt.Errorf("update backup routine %q: %w", name, ErrNotFound)
	}
	c.backupConfig.BackupRoutines[name] = r
	c.backupConfig.invalidatedRoutines = append(c.backupConfig.invalidatedRoutines, name)

	return nil
}

func (c *Config) AddCluster(name string, cluster *AerospikeCluster) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.AerospikeClusters[name]; exists {
		return fmt.Errorf("add Aerospike cluster %q: %w", name, ErrAlreadyExists)
	}
	c.backupConfig.AerospikeClusters[name] = cluster
	return nil
}

func (c *Config) DeleteCluster(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cluster, exists := c.backupConfig.AerospikeClusters[name]
	if !exists {
		return fmt.Errorf("delete Aerospike cluster %q: %w", name, ErrNotFound)
	}
	if routine := c.routineUsesCluster(cluster); routine != "" {
		return fmt.Errorf("delete Aerospike cluster %q: %w: it is used in routine %q", name, ErrInUse, routine)
	}
	delete(c.backupConfig.AerospikeClusters, name)

	return nil
}

func (c *Config) UpdateCluster(name string, cluster *AerospikeCluster) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.AerospikeClusters[name]; !exists {
		return fmt.Errorf("update Aerospike cluster %q: %w", name, ErrNotFound)
	}

	oldCluster := c.backupConfig.AerospikeClusters[name]
	for _, r := range c.backupConfig.BackupRoutines {
		if r.SourceCluster == oldCluster {
			r.SourceCluster = cluster
		}
	}

	c.backupConfig.AerospikeClusters[name] = cluster

	return nil
}

func (c *Config) routineUsesCluster(cluster *AerospikeCluster) string {
	for name, r := range c.backupConfig.BackupRoutines {
		if r.SourceCluster == cluster {
			return name
		}
	}
	return ""
}

func (c *Config) AddSecretAgent(name string, agent *SecretAgent) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.SecretAgents[name]; exists {
		return fmt.Errorf("add Secret agent %q: %w", name, ErrAlreadyExists)
	}
	c.backupConfig.SecretAgents[name] = agent
	return nil
}

func (c *Config) SetBackupConfig(other *BackupConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.backupConfig = *other
	for name := range c.backupConfig.BackupRoutines {
		c.backupConfig.invalidatedRoutines = append(c.backupConfig.invalidatedRoutines, name)
	}
}

// ToggleRoutineDisabled sets the Disabled field of the BackupRoutine based on the provided state.
func (c *Config) ToggleRoutineDisabled(name string, isDisabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, exists := c.backupConfig.BackupRoutines[name]
	if !exists {
		return fmt.Errorf("toggle disable for backup routine %q: %w", name, ErrNotFound)
	}

	c.backupConfig.BackupRoutines[name].Disabled = isDisabled
	if !isDisabled { // only invalidate if we are enabling the routine
		c.backupConfig.invalidatedRoutines = append(c.backupConfig.invalidatedRoutines, name)
	}
	return nil
}

func (c *Config) PopInvalidatedRoutines() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := c.backupConfig.invalidatedRoutines
	c.backupConfig.invalidatedRoutines = nil
	return result
}
