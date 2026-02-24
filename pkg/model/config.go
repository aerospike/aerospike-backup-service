package model

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
)

// Config represents the service configuration.
type Config struct {
	mu            sync.RWMutex
	backupConfig  BackupConfig
	ServiceConfig ServiceConfig
}

type BackupConfig struct {
	AerospikeClusters   map[string]*AerospikeCluster
	Storage             map[string]Storage // Storage is an interface
	BackupPolicies      map[string]*BackupPolicy
	BackupRoutines      map[string]*BackupRoutine
	SecretAgents        map[string]*SecretAgent
	invalidatedRoutines map[string]struct{} // set of routines that need to be rescanned after a change
}

func (bc *BackupConfig) copy() *BackupConfig {
	if bc == nil {
		return nil
	}

	newConfig := &BackupConfig{
		AerospikeClusters: maps.Clone(bc.AerospikeClusters),
		Storage:           maps.Clone(bc.Storage),
		BackupPolicies:    maps.Clone(bc.BackupPolicies),
		BackupRoutines:    maps.Clone(bc.BackupRoutines),
		SecretAgents:      maps.Clone(bc.SecretAgents),
	}

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
	ErrAlreadyExists = errors.New("item already exists")
	ErrNotFound      = errors.New("item not found")
	ErrInUse         = errors.New("item is in use")
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
	for _, r := range c.backupConfig.BackupRoutines {
		if r.Storage == oldStorage {
			r.Storage = s
			c.invalidateRoutine(r.Name)
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
			c.invalidateRoutine(r.Name)
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

func (c *Config) Routine(name string) (*BackupRoutine, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	routine, found := c.backupConfig.BackupRoutines[name]
	if !found {
		return nil, false
	}

	return routine, true
}

func (c *Config) AddRoutine(r *BackupRoutine) error {
	if r.Name == "" {
		return errors.New("backup routine name is empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.BackupRoutines[r.Name]; exists {
		return fmt.Errorf("add backup routine %q: %w", r.Name, ErrAlreadyExists)
	}
	c.backupConfig.BackupRoutines[r.Name] = r
	c.invalidateRoutine(r.Name)

	return nil
}

func (c *Config) DeleteRoutine(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.BackupRoutines[name]; !exists {
		return fmt.Errorf("delete backup routine %q: %w", name, ErrNotFound)
	}
	c.invalidateRoutine(name)
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
	c.invalidateRoutine(r.Name)

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
			c.invalidateRoutine(r.Name)
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
	for _, r := range c.backupConfig.BackupRoutines {
		c.invalidateRoutine(r.Name)
	}
}

// ToggleRoutineDisabled sets the Disabled field of the BackupRoutine based on the provided state.
func (c *Config) ToggleRoutineDisabled(name string, isDisabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	routine, exists := c.backupConfig.BackupRoutines[name]
	if !exists {
		return fmt.Errorf("toggle disable for backup routine %q: %w", name, ErrNotFound)
	}

	c.backupConfig.BackupRoutines[name].Disabled = isDisabled
	c.invalidateRoutine(routine.Name)

	return nil
}

// PopInvalidatedRoutineNames returns all invalidated routine names since the last call.
func (c *Config) PopInvalidatedRoutineNames() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	names := slices.Sorted(maps.Keys(c.backupConfig.invalidatedRoutines))

	// Drain invalidations so the next call only returns newly invalidated routines.
	clear(c.backupConfig.invalidatedRoutines)

	return names
}

// invalidateRoutine marks a routine as invalidated.
// Caller must hold c.mu.
func (c *Config) invalidateRoutine(name string) {
	if c.backupConfig.invalidatedRoutines == nil {
		c.backupConfig.invalidatedRoutines = make(map[string]struct{})
	}

	c.backupConfig.invalidatedRoutines[name] = struct{}{}
}
