package dto

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
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

// remaps reports whether namespace maps the backup's namespace onto another one. The server reads
// a backup under the namespace it restores into, so it finds no backup there.
func remaps(namespace *model.RestoreNamespace) bool {
	return namespace != nil && namespace.Destination != "" && namespace.Destination != namespace.Source
}

// validateRestorePolicy checks policy against the service's backup mode: a server-side restore
// must choose a mode and sets no option the server has no equivalent for, and a scan restore has
// no mode.
func validateRestorePolicy(policy *model.RestorePolicy, config *model.Config) error {
	server := config.ServiceConfig.GetBackupCommonOrDefault().GetBackupModeOrDefault() == model.BackupModeServer

	switch {
	case server && policy.Mode == "":
		return errValidationEmptyField("mode")
	case server:
		return rejectServerUnsupported("restore policy",
			serverOption{"bin-list", len(policy.BinList) > 0},
			serverOption{"replace", ptr.ValueOrZero(policy.Replace)},
			serverOption{"unique", ptr.ValueOrZero(policy.Unique)},
			serverOption{"no-generation", ptr.ValueOrZero(policy.NoGeneration)},
			serverOption{"bandwidth", policy.Bandwidth != nil},
			serverOption{"encryption", encrypts(policy.EncryptionPolicy)},
			serverOption{"compression", compresses(policy.CompressionPolicy)},
			serverOption{"namespace into another namespace", remaps(policy.Namespace)},
		)
	case policy.Mode != "":
		return fmt.Errorf("%w: \"mode\" applies only to backup-mode %q", errInvalidValue, model.BackupModeServer)
	default:
		return nil
	}
}
