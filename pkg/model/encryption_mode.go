package model

// EncryptionMode identifies the encryption algorithm used for backup files.
type EncryptionMode string

const (
	EncryptionModeNone   EncryptionMode = "NONE"
	EncryptionModeAES128 EncryptionMode = "AES128"
	EncryptionModeAES256 EncryptionMode = "AES256"
)
