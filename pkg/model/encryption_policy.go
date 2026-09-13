package model

import "github.com/aerospike/aerospike-backup-service/v3/pkg/redact"

// EncryptionPolicy contains backup encryption information.
type EncryptionPolicy struct {
	// The encryption mode to be used (NONE, AES128, AES256)
	Mode EncryptionMode
	// The path to the file containing the encryption key.
	KeyFile string
	// The name of the environment variable containing the encryption key.
	KeyEnv string
	// The secret keyword in Aerospike Secret Agent containing the encryption key.
	KeySecret redact.Secret
}
