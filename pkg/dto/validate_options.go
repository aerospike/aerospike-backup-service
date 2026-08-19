package dto

// ValidationOptions configures optional validation behavior passed through Validate methods.
type ValidationOptions uint

const (
	// ValidationDefault is the zero value: full validation with no optional flags enabled.
	ValidationDefault ValidationOptions = 0
)

const (
	// ValidationSkipTLSFiles skips filesystem existence checks for TLS certificate and key paths.
	// Used when validating config before files are present on disk (for example API config updates).
	ValidationSkipTLSFiles ValidationOptions = 1 << iota

	// ValidationAllowEmpty permits optional override fields to be omitted.
	// Used by RestoreTimestampRequest via DestinationClusterConfig and StorageConfig.
	ValidationAllowEmpty

	// ValidationWithSecretAgent enables secret-agent reference validation for secret fields on DTOs
	// that do not embed SecretAgentConfig (for example EncryptionPolicy and TLS).
	ValidationWithSecretAgent

	// ValidationWithTLS requires tls-name on seed nodes when the cluster has a TLS block configured.
	// Set by AerospikeCluster when validating SeedNode entries.
	ValidationWithTLS
)

// Has reports whether all bits in flags are set in o.
func (o ValidationOptions) Has(flags ValidationOptions) bool {
	return o&flags == flags
}

// With returns o with flags set.
func (o ValidationOptions) With(flags ValidationOptions) ValidationOptions {
	return o | flags
}
