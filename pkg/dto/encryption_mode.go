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

func (m EncryptionMode) normalized() EncryptionMode {
	if m == "" {
		return m
	}
	return EncryptionMode(foldUpper(string(m)))
}

// Validate checks that the encryption mode is supported.
func (m EncryptionMode) Validate() error {
	switch m.normalized() {
	case EncryptionModeNone, EncryptionModeAES128, EncryptionModeAES256:
		return nil
	default:
		return errValidationInvalidValue(
			"mode",
			m,
			[]EncryptionMode{EncryptionModeNone, EncryptionModeAES128, EncryptionModeAES256},
		)
	}
}

// ToModel converts the DTO encryption mode to the model type.
func (m EncryptionMode) ToModel() model.EncryptionMode {
	return model.EncryptionMode(m.normalized())
}

// NewEncryptionModeFromModel creates a DTO encryption mode from the model type.
func NewEncryptionModeFromModel(m model.EncryptionMode) EncryptionMode {
	return EncryptionMode(m)
}
