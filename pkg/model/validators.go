package model

import (
	"errors"
	"fmt"
)

// Validate validates the configuration.
func (c *Config) Validate() error {
	for _, routine := range c.BackupRoutines {
		if err := routine.Validate(); err != nil {
			return err
		}
	}
	for _, storage := range c.Storage {
		if err := storage.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate validates the Aerospike cluster entity.
func (c *AerospikeCluster) Validate() error {
	if c == nil {
		return errors.New("cluster is not specified")
	}
	if c.Host == nil {
		return errors.New("host is not specified")
	}
	if c.Port == nil {
		return errors.New("port is not specified")
	}
	return nil
}

// Validate validates the storage configuration.
func (s *Storage) Validate() error {
	if s == nil {
		return errors.New("source storage is not specified")
	}
	if s.Path == nil {
		return errors.New("storage path is required")
	}
	return nil
}

// Validate validates the backup routine configuration.
func (r *BackupRoutine) Validate() error {
	if r.BackupPolicy == "" {
		return routineValidationError("backup-policy")
	}
	if r.SourceCluster == "" {
		return routineValidationError("source-cluster")
	}
	if r.Storage == "" {
		return routineValidationError("storage")
	}
	if r.Namespace == nil {
		return routineValidationError("namespace")
	}
	if r.IntervalMillis == nil && r.IncrIntervalMillis == nil {
		return errors.New("interval or incr-interval must be specified for backup routine")
	}
	return nil
}

func routineValidationError(field string) error {
	return fmt.Errorf("%s specification for backup routine is required", field)
}

// Validate validates the restore operation request.
func (r *RestoreRequest) Validate() error {
	if r.DestinationCuster == nil {
		return errors.New("destination cluster is not specified")
	}
	if err := r.DestinationCuster.Validate(); err != nil {
		return err
	}
	if r.Policy == nil {
		return errors.New("restore policy is not specified")
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	if r.DestinationCuster == nil {
		return errors.New("destination cluster is not specified")
	}
	if err := r.SourceStorage.Validate(); err != nil {
		return err
	}
	if r.Policy == nil {
		return errors.New("restore policy is not specified")
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	return nil
}

// Validate validates the restore operation request.
func (r *RestoreTimestampRequest) Validate() error {
	if r.DestinationCuster == nil {
		return errors.New("destination cluster is not specified")
	}
	if err := r.DestinationCuster.Validate(); err != nil {
		return err
	}
	if r.Policy == nil {
		return errors.New("restore policy is not specified")
	}
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	if r.Time == 0 {
		return errors.New("restore point in time is not specified")
	}
	if r.Routine == "" {
		return errors.New("routine to restore is not specified")
	}
	return nil
}

func (p *RestorePolicy) Validate() error {
	return nil
}
