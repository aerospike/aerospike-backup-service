package model

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/pkg/util"
)

// Config represents the service configuration.
type Config struct {
	ServiceConfig     *BackupServiceConfig
	AerospikeClusters map[string]*AerospikeCluster
	Storage           map[string]*Storage
	BackupPolicies    map[string]*BackupPolicy
	BackupRoutines    map[string]*BackupRoutine
	SecretAgents      map[string]*SecretAgent
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

func (c *Config) AddStorage(name string, newStorage *Storage) error {
	_, found := c.Storage[name]
	if found {
		return fmt.Errorf("storage %s already exists", name)
	}

	c.Storage[name] = newStorage
	return nil
}

func (c *Config) DeleteStorage(name string) error {
	toDelete, found := c.Storage[name]
	if !found {
		return fmt.Errorf("storage %s not found", name)
	}
	routine := util.Find(c.BackupRoutines, func(routine *BackupRoutine) bool {
		return routine.Storage == toDelete
	})
	if routine != nil {
		return fmt.Errorf("cannot delete storage as it is used in a routine %s", *routine)
	}

	delete(c.Storage, name)
	return nil
}

// UpdateStorage updates an existing Storage in the configuration.
func (c *Config) UpdateStorage(name string, updatedStorage *Storage) error {
	_, found := c.Storage[name]
	if !found {
		return fmt.Errorf("storage %s not found", name)
	}

	c.Storage[name] = updatedStorage
	return nil
}
