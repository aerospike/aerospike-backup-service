package model

import (
	"fmt"
	"sync"
)

// Config represents the service configuration.
type Config struct {
	mu                sync.Mutex
	ServiceConfig     BackupServiceConfig
	AerospikeClusters map[string]*AerospikeCluster
	Storage           map[string]Storage // Storage is an interface
	BackupPolicies    map[string]*BackupPolicy
	BackupRoutines    map[string]*BackupRoutine
	SecretAgents      map[string]*SecretAgent
}

func NewConfig() *Config {
	return &Config{
		AerospikeClusters: make(map[string]*AerospikeCluster),
		Storage:           make(map[string]Storage),
		BackupPolicies:    make(map[string]*BackupPolicy),
		BackupRoutines:    make(map[string]*BackupRoutine),
		SecretAgents:      make(map[string]*SecretAgent),
	}
}

var (
	ErrAlreadyExists = fmt.Errorf("item already exists")
	ErrNotFound      = fmt.Errorf("item not found")
	ErrInUse         = fmt.Errorf("item is in use")
)

func (c *Config) AddStorage(name string, s Storage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.Storage[name]; exists {
		return fmt.Errorf("add storage %q: %w", name, ErrAlreadyExists)
	}
	c.Storage[name] = s
	return nil
}

func (c *Config) DeleteStorage(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	s, exists := c.Storage[name]
	if !exists {
		return fmt.Errorf("delete storage %q: %w", name, ErrNotFound)
	}
	if routine := c.routineUsesStorage(s); routine != "" {
		return fmt.Errorf("delete storage %q: %w: it is used in routine %q", name, ErrInUse, routine)
	}
	delete(c.Storage, name)
	return nil
}

func (c *Config) UpdateStorage(name string, s Storage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.Storage[name]; !exists {
		return fmt.Errorf("update storage %q: %w", name, ErrNotFound)
	}

	oldStorage := c.Storage[name]
	for _, r := range c.BackupRoutines {
		if r.Storage == oldStorage {
			r.Storage = s
		}
	}

	c.Storage[name] = s

	return nil
}

func (c *Config) routineUsesStorage(s Storage) string {
	for name, r := range c.BackupRoutines {
		if r.Storage == s {
			return name
		}
	}
	return ""
}

func (c *Config) AddPolicy(name string, p *BackupPolicy) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.BackupPolicies[name]; exists {
		return fmt.Errorf("add backup policy %q: %w", name, ErrAlreadyExists)
	}
	c.BackupPolicies[name] = p
	return nil
}

func (c *Config) DeletePolicy(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	p, exists := c.BackupPolicies[name]
	if !exists {
		return fmt.Errorf("delete backup policy %q: %w", name, ErrNotFound)
	}
	if routine := c.routineUsesPolicy(p); routine != "" {
		return fmt.Errorf("delete backup policy %q: %w: it is used in routine %q", name, ErrInUse, routine)
	}
	delete(c.BackupPolicies, name)
	return nil
}

func (c *Config) UpdatePolicy(name string, p *BackupPolicy) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.BackupPolicies[name]; !exists {
		return fmt.Errorf("update backup policy %q: %w", name, ErrNotFound)
	}

	oldPolicy := c.BackupPolicies[name]
	for _, r := range c.BackupRoutines {
		if r.BackupPolicy == oldPolicy {
			r.BackupPolicy = p
		}
	}
	c.BackupPolicies[name] = p

	return nil
}

func (c *Config) routineUsesPolicy(p *BackupPolicy) string {
	for name, r := range c.BackupRoutines {
		if r.BackupPolicy == p {
			return name
		}
	}
	return ""
}

func (c *Config) AddRoutine(name string, r *BackupRoutine) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.BackupRoutines[name]; exists {
		return fmt.Errorf("add backup routine %q: %w", name, ErrAlreadyExists)
	}
	c.BackupRoutines[name] = r
	return nil
}

func (c *Config) DeleteRoutine(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.BackupRoutines[name]; !exists {
		return fmt.Errorf("delete backup routine %q: %w", name, ErrNotFound)
	}
	delete(c.BackupRoutines, name)
	return nil
}

func (c *Config) UpdateRoutine(name string, r *BackupRoutine) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.BackupRoutines[name]; !exists {
		return fmt.Errorf("update backup routine %q: %w", name, ErrNotFound)
	}
	c.BackupRoutines[name] = r
	return nil
}

func (c *Config) AddCluster(name string, cluster *AerospikeCluster) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.AerospikeClusters[name]; exists {
		return fmt.Errorf("add Aerospike cluster %q: %w", name, ErrAlreadyExists)
	}
	c.AerospikeClusters[name] = cluster
	return nil
}

func (c *Config) DeleteCluster(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cluster, exists := c.AerospikeClusters[name]
	if !exists {
		return fmt.Errorf("delete Aerospike cluster %q: %w", name, ErrNotFound)
	}
	if routine := c.routineUsesCluster(cluster); routine != "" {
		return fmt.Errorf("delete Aerospike cluster %q: %w: it is used in routine %q", name, ErrInUse, routine)
	}
	delete(c.AerospikeClusters, name)
	return nil
}

func (c *Config) UpdateCluster(name string, cluster *AerospikeCluster) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.AerospikeClusters[name]; !exists {
		return fmt.Errorf("update Aerospike cluster %q: %w", name, ErrNotFound)
	}

	oldCluster := c.AerospikeClusters[name]
	for _, r := range c.BackupRoutines {
		if r.SourceCluster == oldCluster {
			r.SourceCluster = cluster
		}
	}

	c.AerospikeClusters[name] = cluster

	return nil
}

func (c *Config) routineUsesCluster(cluster *AerospikeCluster) string {
	for name, r := range c.BackupRoutines {
		if r.SourceCluster == cluster {
			return name
		}
	}
	return ""
}

func (c *Config) AddSecretAgent(name string, agent *SecretAgent) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.SecretAgents[name]; exists {
		return fmt.Errorf("add Secret agent %q: %w", name, ErrAlreadyExists)
	}
	c.SecretAgents[name] = agent
	return nil
}
