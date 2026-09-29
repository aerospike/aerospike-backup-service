package model

import (
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
)

// EncryptionPolicy contains backup encryption information.
type EncryptionPolicy struct {
	// The encryption mode to be used (NONE, AES128, AES256)
	Mode EncryptionMode
	// The path to the file containing the encryption key.
	KeyFile string
	// The name of the environment variable containing the encryption key.
	KeyEnv string
	// The secret keyword in Aerospike Secret Agent containing the encryption key.
	KeySecret Secret
}

// ToLibraryPolicy converts the policy into its backup-go representation.
// A nil policy yields nil, which backup-go reads as "no encryption".
func (p *EncryptionPolicy) ToLibraryPolicy() *backup.EncryptionPolicy {
	if p == nil {
		return nil
	}

	return &backup.EncryptionPolicy{
		Mode:      p.Mode.String(),
		KeyFile:   ptr.StringOrNil(p.KeyFile),
		KeySecret: ptr.StringOrNil(p.KeySecret.Reveal()),
		KeyEnv:    ptr.StringOrNil(p.KeyEnv),
	}
}

// ValidateCanDecrypt reports whether this policy can read data written with mode.
// Data written in the clear needs no policy at all; encrypted data needs a policy that
// names the same mode and carries a key. An absent policy is the no-encryption case.
func (p *EncryptionPolicy) ValidateCanDecrypt(mode EncryptionMode) error {
	if !mode.IsEncrypted() {
		return nil
	}

	if p == nil {
		return fmt.Errorf("backup is encrypted with mode '%s', "+
			"but no encryption policy was provided in the restore request", mode)
	}

	if p.Mode != mode {
		return fmt.Errorf("backup is encrypted with mode '%s', "+
			"but the provided encryption policy specifies mode '%s'", mode, p.Mode)
	}

	if !p.hasKey() {
		return errors.New("backup is encrypted, " +
			"but no encryption key (KeyFile, KeyEnv, or KeySecret) was provided in the encryption policy")
	}

	return nil
}

// hasKey reports whether the policy carries an encryption key in any of its forms.
func (p *EncryptionPolicy) hasKey() bool {
	return p.KeyFile != "" || p.KeyEnv != "" || p.KeySecret != ""
}
