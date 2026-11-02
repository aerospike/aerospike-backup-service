package dto

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// RestoreMode selects how the server restores a server-side backup.
// @Description RestoreMode selects how the server restores a server-side backup.
type RestoreMode string

const (
	// RestoreModeStrict restores cold: the namespace is taken from clients, cleared and
	// hydrated from the backups.
	RestoreModeStrict RestoreMode = "strict"
	// RestoreModeFuzzy writes the backed-up records into the live namespace.
	RestoreModeFuzzy RestoreMode = "fuzzy"
)

var restoreModes = []RestoreMode{RestoreModeStrict, RestoreModeFuzzy}

// Validate checks that the restore mode is supported.
func (m RestoreMode) Validate() error {
	if m == "" {
		return nil
	}
	if _, ok := canonicalEnum(m, restoreModes); ok {
		return nil
	}

	return errValidationInvalidValue("mode", m, restoreModes)
}

// ToModel converts the DTO restore mode to the model type.
func (m RestoreMode) ToModel() model.RestoreMode {
	c, _ := canonicalEnum(m, restoreModes)
	return model.RestoreMode(c)
}

// validateRestoreMode checks mode against the service's backup mode: a server-side restore
// must choose one, and a scan restore has none.
func validateRestoreMode(mode model.RestoreMode, config *model.Config) error {
	server := config.ServiceConfig.GetBackupCommonOrDefault().GetBackupModeOrDefault() == model.BackupModeServer

	switch {
	case server && mode == "":
		return errValidationEmptyField("mode")
	case !server && mode != "":
		return fmt.Errorf("%w: \"mode\" applies only to backup-mode %q", errInvalidValue, model.BackupModeServer)
	default:
		return nil
	}
}
