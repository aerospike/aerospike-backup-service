package dto

import (
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
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
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty" default:"NONE" enums:"NONE,AES128,AES256"`
	// The path to the file containing the encryption key.
	KeyFile *string `yaml:"key-file,omitempty" json:"key-file,omitempty"`
	// The name of the environment variable containing the encryption key.
	KeyEnv *string `yaml:"key-env,omitempty" json:"key-env,omitempty"`
	// The secret keyword in Aerospike Secret Agent containing the encryption key.
	KeySecret *string `yaml:"key-secret,omitempty" json:"key-secret,omitempty"`
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

func (p *EncryptionPolicy) ToModel() *model.EncryptionPolicy {
	if p == nil {
		return nil
	}

	return &model.EncryptionPolicy{
		Mode:      p.Mode,
		KeyFile:   p.KeyFile,
		KeyEnv:    p.KeyEnv,
		KeySecret: p.KeySecret,
	}
}

func (p *EncryptionPolicy) FromModel(m *model.EncryptionPolicy) {
	p.Mode = m.Mode
	p.KeyFile = m.KeyFile
	p.KeyEnv = m.KeyEnv
	p.KeySecret = m.KeySecret
}
