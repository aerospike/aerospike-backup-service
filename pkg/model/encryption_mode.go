package model

// EncryptionMode identifies the encryption algorithm used for backup files.
type EncryptionMode string

const (
	EncryptionModeNone   EncryptionMode = "NONE"
	EncryptionModeAES128 EncryptionMode = "AES128"
	EncryptionModeAES256 EncryptionMode = "AES256"
)

// String returns the wire value of the encryption mode.
func (m EncryptionMode) String() string {
	return string(m)
}

// IsEncrypted reports whether the mode denotes actual encryption. Both an unset mode and
// EncryptionModeNone mean the data is stored in the clear.
func (m EncryptionMode) IsEncrypted() bool {
	return m != "" && m != EncryptionModeNone
}
