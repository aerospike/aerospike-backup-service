package dto

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
)

// TLS represents the Aerospike cluster TLS configuration options.
// @Description TLS represents the Aerospike cluster TLS configuration options.
//
//nolint:lll
type TLS struct {
	// Path to a trusted CA certificate file.
	CAFile *string `yaml:"ca-file,omitempty" json:"ca-file,omitempty" example:"/path/to/cafile.pem" extensions:"x-nullable"`
	// Path to a directory of trusted CA certificates.
	CAPath *string `yaml:"ca-path,omitempty" json:"ca-path,omitempty" example:"/path/to/ca" extensions:"x-nullable"`
	// TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.
	Protocols *string `yaml:"protocols,omitempty" json:"protocols,omitempty" default:"TLSv1.2"`
	// TLS cipher selection criteria. The format is the same as OpenSSL's Cipher List Format.
	CipherSuite *string `yaml:"cipher-suite,omitempty" json:"cipher-suite,omitempty" example:"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA" extensions:"x-nullable"`

	// mTLS configuration:

	// The default TLS name used to authenticate each TLS socket connection.
	Name *string `yaml:"name,omitempty" json:"name,omitempty" example:"tls-name" extensions:"x-nullable"`
	// Path to the key for mutual authentication (if Aerospike cluster supports it).
	Keyfile *string `yaml:"key-file,omitempty" json:"key-file,omitempty" example:"/path/to/keyfile.pem" extensions:"x-nullable"`
	// Password to load protected TLS-keyfile (env:VAR, file:PATH, PASSWORD).
	KeyfilePassword *string `yaml:"key-file-password,omitempty" json:"key-file-password,omitempty" example:"file:/path/to/password" extensions:"x-nullable"`
	// Path to the chain file for mutual authentication (if Aerospike Cluster supports it).
	Certfile *string `yaml:"cert-file,omitempty" json:"cert-file,omitempty" example:"/path/to/certfile.pem" extensions:"x-nullable"`
}

func (t *TLS) Validate() error {
	if t == nil {
		return nil // TLS is optional
	}

	if err := t.validateCACertificates(); err != nil {
		return err
	}

	if err := t.validateMutualTLS(); err != nil {
		return err
	}

	if err := t.validateKeyfilePassword(); err != nil {
		return err
	}

	return t.validateTLSConfig()
}

// validateCACertificates ensures CA file and path are mutually exclusive.
func (t *TLS) validateCACertificates() error {
	if hasText(t.CAFile) && hasText(t.CAPath) {
		return errValidationMutuallyExclusive("ca-file", "ca-path")
	}
	if !hasText(t.CAFile) && !hasText(t.CAPath) {
		return errValidationRequiredEither("ca-file", "ca-path")
	}

	return nil
}

// validateMutualTLS ensures that if any mTLS field is set, all required fields are present.
func (t *TLS) validateMutualTLS() error {
	mtlsAnySet := hasText(t.Name) || hasText(t.Keyfile) || hasText(t.Certfile)

	if !mtlsAnySet {
		return nil // No mTLS fields set, which is valid
	}

	// If any mTLS field is set, all must be set
	if !hasText(t.Name) {
		return errValidationRequires("cert-file/key-file", "name")
	}
	if !hasText(t.Keyfile) {
		return errValidationRequires("name/cert-file", "key-file")
	}
	if !hasText(t.Certfile) {
		return errValidationRequires("name/key-file", "cert-file")
	}

	return nil
}

// validateKeyfilePassword ensures keyfile password is only set when keyfile is present.
func (t *TLS) validateKeyfilePassword() error {
	if hasText(t.KeyfilePassword) && !hasText(t.Keyfile) {
		return errValidationRequires("key-file-password", "key-file")
	}

	return nil
}

// validateTLSConfig attempts to create a TLS config to catch low-level issues.
func (t *TLS) validateTLSConfig() error {
	if _, err := aerospike.NewTLSConfig(t.toModel()); err != nil {
		return fmt.Errorf("tls: invalid configuration: %w", err)
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
	t.KeyfilePassword = m.KeyfilePassword
	t.Certfile = m.Certfile
}

func (t *TLS) toModel() *model.TLS {
	if t == nil {
		return nil
	}

	return &model.TLS{
		CAFile:          t.CAFile,
		CAPath:          t.CAPath,
		Name:            t.Name,
		Protocols:       t.Protocols,
		CipherSuite:     t.CipherSuite,
		Keyfile:         t.Keyfile,
		KeyfilePassword: t.KeyfilePassword,
		Certfile:        t.Certfile,
	}
}
