package dto

import (
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/pkg/model"
)

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
	Mode string `json:"mode,omitempty" default:"NONE" enums:"NONE,AES128,AES256"`
	// The path to the file containing the encryption key.
	KeyFile *string `json:"key-file,omitempty"`
	// The name of the environment variable containing the encryption key.
	KeyEnv *string `json:"key-env,omitempty"`
	// The secret keyword in Aerospike Secret Agent containing the encryption key.
	KeySecret *string `json:"key-secret,omitempty"`
}

// Validate validates the encryption policy.
func (p *EncryptionPolicy) Validate() error {
	if p == nil {
		return nil
	}
	if p.Mode != EncryptNone && p.Mode != EncryptAES128 && p.Mode != EncryptAES256 {
		return fmt.Errorf("invalid encryption mode: %s", p.Mode)
	}
	if p.KeyFile == nil && p.KeyEnv == nil && p.KeySecret == nil {
		return errors.New("encryption key location not specified")
	}
	return nil
}

func MapEncryptionPolicyFromDTO(dto EncryptionPolicy) model.EncryptionPolicy {
	return model.EncryptionPolicy{
		Mode:      dto.Mode,
		KeyFile:   dto.KeyFile,
		KeyEnv:    dto.KeyEnv,
		KeySecret: dto.KeySecret,
	}
}
