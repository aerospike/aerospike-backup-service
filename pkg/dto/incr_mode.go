package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// IncrMode represents the mode for incremental backups.
// @Description IncrMode represents the mode for incremental backups.
type IncrMode string

const (
	// IncrModeDifferential is the default mode. It backs up data since the last successful backup (full or incremental).
	IncrModeDifferential IncrMode = "differential"
	// IncrModeCumulative backs up data since the last successful full backup.
	IncrModeCumulative IncrMode = "cumulative"
)

var incrModes = []IncrMode{IncrModeDifferential, IncrModeCumulative}

// Validate checks that the incremental mode is supported.
func (m IncrMode) Validate() error {
	if m == "" {
		return nil
	}
	if _, ok := canonicalEnum(m, incrModes); ok {
		return nil
	}

	return errValidationInvalidValue("incr-mode", m, incrModes)
}

// ToModel converts the DTO incremental mode to the model type.
func (m IncrMode) ToModel() model.IncrMode {
	if m == "" {
		return model.IncrModeDifferential
	}
	c, _ := canonicalEnum(m, incrModes)
	return model.IncrMode(c)
}

// NewIncrModeFromModel creates a DTO incremental mode from the model type.
func NewIncrModeFromModel(m model.IncrMode) IncrMode {
	if m == "" {
		return IncrModeDifferential
	}
	return IncrMode(m)
}
