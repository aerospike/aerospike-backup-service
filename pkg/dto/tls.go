package dto

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/tlsconfig"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/safepath"
)

// TLS represents the Aerospike cluster TLS configuration options.
// @Description TLS represents the Aerospike cluster TLS configuration options.
//
//nolint:lll
type TLS struct {
	ClientTLS `yaml:",inline"`
	// Path to a directory of trusted CA certificates.
	CAPath string `yaml:"ca-path,omitempty" json:"ca-path,omitempty" example:"/path/to/ca" extensions:"x-nullable"`
	// TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.
	Protocols string `yaml:"protocols,omitempty" json:"protocols,omitempty" default:"TLSv1.2"`
	// TLS cipher selection criteria. The format is the same as OpenSSL's Cipher List Format.
	CipherSuite string `yaml:"cipher-suite,omitempty" json:"cipher-suite,omitempty" example:"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA" extensions:"x-nullable"`
	// Passphrase for an encrypted TLS key file.
	// This is sensitive information. Can be a path in secret agent or an actual value.
	// Literal values are redacted as "[secret]" in API responses; secret agent references are returned as-is.
	KeyfilePassword secret `yaml:"key-file-password,omitempty" json:"key-file-password,omitempty" format:"password" extensions:"x-nullable"`
}

func (t *TLS) Validate(opts ValidationOptions) error {
	if t == nil {
		return nil // TLS is optional
	}

	if err := t.ClientTLS.Validate(opts); err != nil {
		return err
	}

	if err := t.validatePaths(); err != nil {
		return err
	}

	if err := t.validateCACertificates(); err != nil {
		return err
	}

	if err := t.validateKeyfilePassword(opts); err != nil {
		return err
	}

	return t.validateTLSConfig(opts)
}

func (t *TLS) validatePaths() error {
	if err := safepath.ValidateClean(t.CAPath); err != nil {
		return errValidationInvalidPath("ca-path", t.CAPath, err)
	}

	return nil
}

// validateCACertificates ensures CA file and path are mutually exclusive.
// Both are optional ways to trust the server certificate; see ClientTLS.Validate.
func (t *TLS) validateCACertificates() error {
	if t.CAFile != "" && t.CAPath != "" {
		return errValidationMutuallyExclusive("ca-file", "ca-path")
	}

	return nil
}

// validateKeyfilePassword ensures keyfile password is only set when keyfile is present.
func (t *TLS) validateKeyfilePassword(opts ValidationOptions) error {
	if t.KeyfilePassword != "" && t.Keyfile == "" {
		return errValidationRequires("key-file-password", "key-file")
	}

	if err := t.KeyfilePassword.Validate(opts.Has(ValidationWithSecretAgent)); err != nil {
		return errValidationSecret("key-file-password", err)
	}

	return nil
}

// validateTLSConfig attempts to create a TLS config to catch low-level issues.
func (t *TLS) validateTLSConfig(opts ValidationOptions) error {
	if opts.Has(ValidationSkipTLSFiles) {
		return nil
	}

	if _, err := tlsconfig.NewTlsConfig(t.toModel()); err != nil {
		return fmt.Errorf("tls %w: %w", errValidation, err)
	}

	return nil
}

func (t *TLS) fromModel(m *model.TLS) {
	t.CAFile = m.CAFile
	t.CAPath = m.CAPath
	t.Name = m.Name
	t.Protocols = m.Protocols
	t.CipherSuite = m.CipherSuite
	t.Keyfile = m.Keyfile
	t.KeyfilePassword = secret(m.KeyfilePassword)
	t.Certfile = m.Certfile
}

func (t *TLS) toModel() *model.TLS {
	if t == nil {
		return nil
	}

	return &model.TLS{
		ClientTLS:       t.ToModel(),
		CAPath:          t.CAPath,
		Protocols:       t.Protocols,
		CipherSuite:     t.CipherSuite,
		KeyfilePassword: string(t.KeyfilePassword),
	}
}
