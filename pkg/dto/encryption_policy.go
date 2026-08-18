package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// Encryption modes.
const (
	EncryptNone   = "NONE"
	EncryptAES128 = "AES128"
	EncryptAES256 = "AES256"
)

// EncryptionPolicy contains backup encryption information.
// @Description EncryptionPolicy contains backup encryption information.
type EncryptionPolicy struct {
	// The encryption mode to be used (NONE, AES128, AES256)
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty" default:"NONE" enums:"NONE,AES128,AES256"`
	// The path to the file containing the encryption key.
	KeyFile string `yaml:"key-file,omitempty" json:"key-file,omitempty" extensions:"x-nullable"`
	// The name of the environment variable containing the encryption key.
	KeyEnv string `yaml:"key-env,omitempty" json:"key-env,omitempty" extensions:"x-nullable"`
	// The secret keyword in Aerospike Secret Agent containing the encryption key.
	// This is sensitive information. Can be a path in secret agent or an actual value.
	// Literal values are redacted as "[secret]" in API responses; secret agent references are returned as-is.
	KeySecret secret `yaml:"key-secret,omitempty" json:"key-secret,omitempty" format:"password" extensions:"x-nullable"`
}

// Validate validates the encryption policy.
func (p *EncryptionPolicy) Validate() error {
	if p == nil {
		return nil
	}
	if p.Mode == "" {
		return errValidationEmptyField("mode")
	}
	if p.Mode != EncryptNone && p.Mode != EncryptAES128 && p.Mode != EncryptAES256 {
		return errValidationInvalidValue("mode", p.Mode, "NONE,AES128,AES256")
	}

	if p.Mode == EncryptNone {
		if p.KeyFile != "" {
			return errValidationMutuallyExclusive("key-file", "mode = NONE")
		}
		if p.KeyEnv != "" {
			return errValidationMutuallyExclusive("key-env", "mode = NONE")
		}
		if p.KeySecret != "" {
			return errValidationMutuallyExclusive("key-secret", "mode = NONE")
		}

		return nil // no more validation for mode = NONE
	}

	if p.KeyFile == "" && p.KeyEnv == "" && p.KeySecret == "" {
		return errValidationRequiredEither("key-file", "key-env", "key-secret")
	}

	// Only one parameter allowed to be set.
	if p.KeyFile != "" && p.KeyEnv != "" {
		return errValidationMutuallyExclusive("key-file", "key-env")
	}
	if p.KeyFile != "" && p.KeySecret != "" {
		return errValidationMutuallyExclusive("key-file", "key-secret")
	}
	if p.KeyEnv != "" && p.KeySecret != "" {
		return errValidationMutuallyExclusive("key-env", "key-secret")
	}

	return nil
}

func (p *EncryptionPolicy) ToModel() *model.EncryptionPolicy {
	if p == nil {
		return nil
	}

	return &model.EncryptionPolicy{
		Mode:      p.Mode,
		KeyFile:   p.KeyFile,
		KeyEnv:    p.KeyEnv,
		KeySecret: string(p.KeySecret),
	}
}

func newEncryptionPolicyFromModel(m *model.EncryptionPolicy) *EncryptionPolicy {
	if m == nil {
		return nil
	}
	e := &EncryptionPolicy{}
	e.fromModel(m)
	return e
}

func (p *EncryptionPolicy) fromModel(m *model.EncryptionPolicy) {
	p.Mode = m.Mode
	p.KeyFile = m.KeyFile
	p.KeyEnv = m.KeyEnv
	p.KeySecret = secret(m.KeySecret)
}
