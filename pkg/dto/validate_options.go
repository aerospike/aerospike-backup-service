package dto

// ValidationOptions configures optional validation behavior passed through Validate methods.
type ValidationOptions uint

const (
	// ValidationDefault is the zero value: full validation with no optional flags enabled.
	ValidationDefault ValidationOptions = 0
)

const (
	// ValidationAllowEmpty permits optional fields to be omitted.
	// Used by RestoreTimestampRequest via DestinationClusterConfig and StorageConfig,
	// and by Path for fields where an empty path is a valid "not configured" value.
	//
	// Path call sites must pass this flag as an explicit literal rather than forwarding
	// a parent's opts: threading a caller's ValidationAllowEmpty down into a Path would
	// silently relax the required-path check on whatever field it reaches.
	ValidationAllowEmpty ValidationOptions = 1 << iota

	// ValidationWithSecretAgent enables secret-agent reference validation for secret fields on DTOs
	// that do not embed SecretAgentConfig (for example EncryptionPolicy and TLS).
	ValidationWithSecretAgent

	// ValidationWithTLS requires tls-name on seed nodes when the cluster has a TLS block configured.
	// Set by AerospikeCluster when validating SeedNode entries.
	ValidationWithTLS

	// ValidationAllowAbsolutePath permits a Path to be absolute. Used for local filesystem
	// paths (certificates, key files, log files, local storage roots). Object-storage prefixes
	// and backup-data-path must stay storage-relative, so they omit this flag.
	ValidationAllowAbsolutePath
)

// ValidationOptionalLocalFile is the common option set for optional local filesystem paths:
// the field may be omitted, and when set it may be absolute.
const ValidationOptionalLocalFile = ValidationAllowEmpty | ValidationAllowAbsolutePath

// Has reports whether all bits in flags are set in o.
func (o ValidationOptions) Has(flags ValidationOptions) bool {
	return o&flags == flags
}
