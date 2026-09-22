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

func NewConfig() *Config {
	return &Config{
		backupConfig: *newBackupConfig(),
	}
}

func newBackupConfig() *BackupConfig {
	return &BackupConfig{
		AerospikeClusters:   make(map[string]*AerospikeCluster),
		Storage:             make(map[string]Storage),
		BackupPolicies:      make(map[string]*BackupPolicy),
		BackupRoutines:      make(map[string]*BackupRoutine),
		SecretAgents:        make(map[string]*SecretAgent),
		invalidatedRoutines: make(map[string]struct{}),
	}
}

func (bc BackupConfig) copy() BackupConfig {
	return BackupConfig{
		AerospikeClusters:   maps.Clone(bc.AerospikeClusters),
		Storage:             maps.Clone(bc.Storage),
		BackupPolicies:      maps.Clone(bc.BackupPolicies),
		BackupRoutines:      maps.Clone(bc.BackupRoutines),
		SecretAgents:        maps.Clone(bc.SecretAgents),
		invalidatedRoutines: make(map[string]struct{}),
	}
}

// FindStorage and FindCluster read from a snapshot the caller already holds, because rendering
// either one resolves its secret agent by identity against that same snapshot's agents; looking
// the entity up under a separate lock could pair it with a map that no longer holds its agent.
// They are named Find because Storage is a field on this type. Lookups that need no snapshot are
// Routine and Policy on Config.
func (bc *BackupConfig) FindStorage(name string) (Storage, error) {
	storage, ok := bc.Storage[name]
	if !ok {
		return nil, NotFound("storage", name)
	}

	return storage, nil
}

func (bc *BackupConfig) FindCluster(name string) (*AerospikeCluster, error) {
	cluster, ok := bc.AerospikeClusters[name]
	if !ok {
		return nil, NotFound("cluster", name)
	}

	return cluster, nil
}

func (c *Config) BackupConfigCopy() *BackupConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	config := c.backupConfig.copy()
	return &config
}

func (c *Config) AddStorage(name string, s Storage) error {
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

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.Storage[name]; exists {
		return AlreadyExists("storage", name)
	}
	c.backupConfig.Storage[name] = s

	return nil
}

func (c *Config) DeleteStorage(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	s, exists := c.backupConfig.Storage[name]
	if !exists {
		return NotFound("storage", name)
	}
	if routine := c.routineUsesStorage(s); routine != "" {
		return InUse("storage", name, fmt.Sprintf("it is used in routine %q", routine))
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

func (c *Config) AddPolicy(name string, p *BackupPolicy) error {
	if p == nil {
		return errors.New("backup policy cannot be nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.BackupPolicies[name]; exists {
		return AlreadyExists("policy", name)
	}
	c.backupConfig.BackupPolicies[name] = p

	return nil
}

func (c *Config) DeletePolicy(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	p, exists := c.backupConfig.BackupPolicies[name]
	if !exists {
		return NotFound("policy", name)
	}
	if routine := c.routineUsesPolicy(p); routine != "" {
		return InUse("policy", name, fmt.Sprintf("it is used in routine %q", routine))
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

func (c *Config) Routines() map[string]*BackupRoutine {
	c.mu.RLock()
	defer c.mu.RUnlock()

	routines := make(map[string]*BackupRoutine, len(c.backupConfig.BackupRoutines))
	maps.Copy(routines, c.backupConfig.BackupRoutines)

	return routines
}

// Routine and Policy return the named entity, or an EntityError naming what was missing, so a
// caller reports a lookup failure by passing the error on rather than restating what it asked for.
func (c *Config) Routine(name string) (*BackupRoutine, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	routine, ok := c.backupConfig.BackupRoutines[name]
	if !ok {
		return nil, NotFound("routine", name)
	}

	return routine, nil
}

func (c *Config) Policy(name string) (*BackupPolicy, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	policy, ok := c.backupConfig.BackupPolicies[name]
	if !ok {
		return nil, NotFound("policy", name)
	}

	return policy, nil
}

func (c *Config) AddRoutine(r *BackupRoutine) error {
	if r == nil {
		return errors.New("backup routine cannot be nil")
	}

	if r.Name == "" {
		return errors.New("backup routine name is empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.BackupRoutines[r.Name]; exists {
		return AlreadyExists("routine", r.Name)
	}
	c.backupConfig.BackupRoutines[r.Name] = r
	c.invalidateRoutine(r.Name)

	return nil
}

func (c *Config) AddCluster(name string, cluster *AerospikeCluster) error {
	if cluster == nil {
		return errors.New("cluster cannot be nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.backupConfig.AerospikeClusters[name]; exists {
		return AlreadyExists("cluster", name)
	}
	c.backupConfig.AerospikeClusters[name] = cluster

	return nil
}

func (c *Config) DeleteCluster(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cluster, exists := c.backupConfig.AerospikeClusters[name]
	if !exists {
		return NotFound("cluster", name)
	}
	if routine := c.routineUsesCluster(cluster); routine != "" {
		return InUse("cluster", name, fmt.Sprintf("it is used in routine %q", routine))
	}
	delete(c.backupConfig.AerospikeClusters, name)

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
		return AlreadyExists("secret agent", name)
	}
	c.backupConfig.SecretAgents[name] = agent
	return nil
}

func (c *Config) SetBackupConfig(other *BackupConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.backupConfig = *other
}

// InvalidateRoutines marks the given routines as needing reschedule and history rescan.
func (c *Config) InvalidateRoutines(names []string) {
	if len(names) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, name := range names {
		c.invalidateRoutine(name)
	}
}

// InvalidateAllRoutines marks every configured routine as invalidated.
func (c *Config) InvalidateAllRoutines() {
	c.mu.Lock()
	defer c.mu.Unlock()

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
		return NotFound("routine", name)
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
	c.backupConfig.invalidatedRoutines[name] = struct{}{}
}
