package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// EncryptionMode identifies the encryption algorithm used for backup files.
// @Description EncryptionMode identifies the encryption algorithm used for backup files.
type EncryptionMode string

const (
	EncryptionModeNone   EncryptionMode = "NONE"
	EncryptionModeAES128 EncryptionMode = "AES128"
	EncryptionModeAES256 EncryptionMode = "AES256"
)

var encryptionModes = []EncryptionMode{EncryptionModeNone, EncryptionModeAES128, EncryptionModeAES256}

// Validate checks that the encryption mode is supported.
func (m EncryptionMode) Validate() error {
	if _, ok := canonicalEnum(m, encryptionModes); ok {
		return nil
	}

	return errValidationInvalidValue("mode", m, encryptionModes)
}

// ToModel converts the DTO encryption mode to the model type.
func (m EncryptionMode) ToModel() model.EncryptionMode {
	c, _ := canonicalEnum(m, encryptionModes)
	return model.EncryptionMode(c)
}

// NewEncryptionModeFromModel creates a DTO encryption mode from the model type.
func NewEncryptionModeFromModel(m model.EncryptionMode) EncryptionMode {
	return EncryptionMode(m)
}
