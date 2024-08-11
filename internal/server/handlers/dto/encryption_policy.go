package dto

// EncryptionPolicyDTO contains backup encryption information.
// @Description EncryptionPolicyDTO contains backup encryption information.
type EncryptionPolicyDTO struct {
	// The encryption mode to be used (NONE, AES128, AES256)
	Mode string `json:"mode,omitempty" default:"NONE" enums:"NONE,AES128,AES256"`
	// The path to the file containing the encryption key.
	KeyFile *string `json:"key-file,omitempty"`
	// The name of the environment variable containing the encryption key.
	KeyEnv *string `json:"key-env,omitempty"`
	// The secret keyword in Aerospike Secret Agent containing the encryption key.
	KeySecret *string `json:"key-secret,omitempty"`
}
