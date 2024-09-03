package model

// Encryption modes
const (
	EncryptNone   = "NONE"
	EncryptAES128 = "AES128"
	EncryptAES256 = "AES256"
)

// EncryptionPolicy contains backup encryption information.
// @Description EncryptionPolicy contains backup encryption information.
type EncryptionPolicy struct {
	// The encryption mode to be used (NONE, AES128, AES256)
	Mode string
	// The path to the file containing the encryption key.
	KeyFile *string
	// The name of the environment variable containing the encryption key.
	KeyEnv *string
	// The secret keyword in Aerospike Secret Agent containing the encryption key.
	KeySecret *string
}
