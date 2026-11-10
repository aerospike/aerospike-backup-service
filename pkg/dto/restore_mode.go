package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// RestoreMode selects how the server restores a server-side backup.
// @Description RestoreMode selects how the server restores a server-side backup.
type RestoreMode string

const (
	// RestoreModeStrict takes the namespace from clients, clears it and loads it from the
	// backups.
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

// validateRestorePolicy checks policy against the service's backup mode. A scan restore has no
// mode. A server-side restore must choose a mode, sets no option that the server has no
// equivalent for, and sets no option that its mode does not support.
func validateRestorePolicy(policy *model.RestorePolicy, config *model.Config) error {
	server := config.ServiceConfig.GetBackupCommonOrDefault().GetBackupModeOrDefault() == model.BackupModeServer
	if !server {
		return rejectOptions("restore policy", onlyInServerMode, option{"mode", policy.Mode != ""})
	}

	if policy.Mode == "" {
		return errValidationEmptyField("mode")
	}

	err := rejectOptions("restore policy", unsupportedInServerMode,
		option{"bin-list", len(policy.BinList) > 0},
		option{"replace", ptr.ValueOrZero(policy.Replace)},
		option{"unique", ptr.ValueOrZero(policy.Unique)},
		option{"no-generation", ptr.ValueOrZero(policy.NoGeneration)},
		option{"bandwidth", policy.Bandwidth != nil},
		option{"encryption", encrypts(policy.EncryptionPolicy)},
		option{"compression", compresses(policy.CompressionPolicy)},
		option{"a namespace destination other than its source", remaps(policy.Namespace)},
		option{"no-records", ptr.ValueOrZero(policy.NoRecords)},
		option{"disable-batch-writes", ptr.ValueOrZero(policy.DisableBatchWrites)},
		option{"batch-size", policy.BatchSize != nil},
		option{"max-async-batches", policy.MaxAsyncBatches != nil},
		option{"socket-timeout", policy.SocketTimeout != nil},
		option{"total-timeout", policy.TotalTimeout != nil},
	)
	if err != nil || policy.Mode != model.RestoreModeStrict {
		return err
	}

	// The server retries the writes of a fuzzy restore. A strict restore writes no record through
	// the write path, so it has nothing to retry.
	return rejectOptions("restore policy", unsupportedInStrictMode, option{"retry-policy", policy.RetryPolicy != nil})
}
