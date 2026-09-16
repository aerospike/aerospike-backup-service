package model

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
)

// Config is the live service configuration: the backup configuration currently installed,
// the service settings, and the routines a change to either has left needing a reschedule.
type Config struct {
	mu            sync.RWMutex
	backupConfig  BackupConfig
	ServiceConfig ServiceConfig
	// invalidated holds the routines the last configuration changes left needing a
	// reschedule and a history rescan, until PopInvalidatedRoutineNames drains them.
	invalidated map[string]struct{}
}

// BackupConfig is the value part of the configuration: what is configured, with no record of
// what changed. Entries are added and removed here; installing the result into a Config is
// what works out which routines that affected.
type BackupConfig struct {
	AerospikeClusters map[string]*AerospikeCluster
	Storage           map[string]Storage // Storage is an interface
	BackupPolicies    map[string]*BackupPolicy
	BackupRoutines    map[string]*BackupRoutine
	SecretAgents      map[string]*SecretAgent
}

func NewConfig() *Config {
	return &Config{
		backupConfig: *NewBackupConfig(),
		invalidated:  make(map[string]struct{}),
	}
}

// NewBackupConfig returns an empty backup configuration ready to have entries added.
func NewBackupConfig() *BackupConfig {
	return &BackupConfig{
		AerospikeClusters: make(map[string]*AerospikeCluster),
		Storage:           make(map[string]Storage),
		BackupPolicies:    make(map[string]*BackupPolicy),
		BackupRoutines:    make(map[string]*BackupRoutine),
		SecretAgents:      make(map[string]*SecretAgent),
	}
}

func (bc BackupConfig) copy() BackupConfig {
	return BackupConfig{
		AerospikeClusters: maps.Clone(bc.AerospikeClusters),
		Storage:           maps.Clone(bc.Storage),
		BackupPolicies:    maps.Clone(bc.BackupPolicies),
		BackupRoutines:    maps.Clone(bc.BackupRoutines),
		SecretAgents:      maps.Clone(bc.SecretAgents),
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
	config := c.backupConfig.copy()
	return &config
}

func (bc *BackupConfig) AddStorage(name string, s Storage) error {
	if s == nil {
		return errors.New("storage cannot be nil")
	}

	switch storage := s.(type) {
	case *LocalStorage:
		if storage == nil {
			return errors.New("storage cannot be nil")
		}
	case *S3Storage:
		if storage == nil {
			return errors.New("storage cannot be nil")
		}
	case *GcpStorage:
		if storage == nil {
			return errors.New("storage cannot be nil")
		}
	case *AzureStorage:
		if storage == nil {
			return errors.New("storage cannot be nil")
		}
	default:
		return fmt.Errorf("unsupported storage type %T", s)
	}

	if _, exists := bc.Storage[name]; exists {
		return fmt.Errorf("add storage %q: %w", name, ErrAlreadyExists)
	}
	bc.Storage[name] = s

	return nil
}

func (bc *BackupConfig) AddPolicy(name string, p *BackupPolicy) error {
	if p == nil {
		return errors.New("backup policy cannot be nil")
	}

	if _, exists := bc.BackupPolicies[name]; exists {
		return fmt.Errorf("add backup policy %q: %w", name, ErrAlreadyExists)
	}
	bc.BackupPolicies[name] = p

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

func (bc *BackupConfig) AddRoutine(r *BackupRoutine) error {
	if r == nil {
		return errors.New("backup routine cannot be nil")
	}

	if r.Name == "" {
		return errors.New("backup routine name is empty")
	}

	if _, exists := bc.BackupRoutines[r.Name]; exists {
		return fmt.Errorf("add backup routine %q: %w", r.Name, ErrAlreadyExists)
	}
	bc.BackupRoutines[r.Name] = r

	return nil
}

func (bc *BackupConfig) AddCluster(name string, cluster *AerospikeCluster) error {
	if cluster == nil {
		return errors.New("cluster cannot be nil")
	}

	if _, exists := bc.AerospikeClusters[name]; exists {
		return fmt.Errorf("add Aerospike cluster %q: %w", name, ErrAlreadyExists)
	}
	bc.AerospikeClusters[name] = cluster

	return nil
}

func (bc *BackupConfig) AddSecretAgent(name string, agent *SecretAgent) error {
	if _, exists := bc.SecretAgents[name]; exists {
		return fmt.Errorf("add Secret agent %q: %w", name, ErrAlreadyExists)
	}
	bc.SecretAgents[name] = agent
	return nil
}

// SetBackupConfig replaces the backup configuration and marks every routine the replacement changed.
func (c *Config) SetBackupConfig(other *BackupConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, name := range ChangedRoutines(other, &c.backupConfig) {
		c.invalidateRoutine(name)
	}
	c.backupConfig = *other
}

// PopInvalidatedRoutineNames returns all invalidated routine names since the last call.
func (c *Config) PopInvalidatedRoutineNames() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	names := slices.Sorted(maps.Keys(c.invalidated))

	// Drain invalidations so the next call only returns newly invalidated routines.
	clear(c.invalidated)

	return names
}

// invalidateRoutine marks a routine as invalidated.
// Caller must hold c.mu.
func (c *Config) invalidateRoutine(name string) {
	c.invalidated[name] = struct{}{}
}
