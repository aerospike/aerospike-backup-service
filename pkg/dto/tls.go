package dto

import (
	"crypto/tls"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/safepath"
)

const clusterTLSProtocol12 = "TLSv1.2"

var clusterCipherSuites = func() map[string]bool {
	result := make(map[string]bool)
	for _, suite := range append(tls.CipherSuites(), tls.InsecureCipherSuites()...) {
		result[suite.Name] = true
	}
	return result
}()

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
	// Colon-separated IANA TLS 1.2 cipher suite names (not OpenSSL nicknames).
	// The suite must match the certificate key type (RSA vs ECDSA).
	// If omitted, the client offers Go crypto/tls TLS 1.2 defaults:
	// TLS_ECDHE_{ECDSA,RSA}_WITH_AES_128_GCM_SHA256,
	// TLS_ECDHE_{ECDSA,RSA}_WITH_AES_256_GCM_SHA384,
	// TLS_ECDHE_{ECDSA,RSA}_WITH_CHACHA20_POLY1305_SHA256,
	// and ECDHE AES-CBC SHA for compatibility.
	// RSA key-exchange, 3DES, RC4, and CBC-SHA256 are not offered.
	// This field does not select TLS 1.3 suites.
	CipherSuite string `yaml:"cipher-suite,omitempty" json:"cipher-suite,omitempty" example:"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" extensions:"x-nullable"`
	// Passphrase for an encrypted TLS key file.
	// This is sensitive information. Can be a path in secret agent or an actual value.
	// Literal values are redacted as "[secret]" in API responses; secret agent references are returned as-is.
	KeyfilePassword secret `yaml:"key-file-password,omitempty" json:"key-file-password,omitempty" format:"password" extensions:"x-nullable"`
}

func (t *TLS) Validate(opts ValidationOptions) error {
	if t == nil {
		return nil // TLS is optional
	}

	if err := t.ClientTLS.Validate(); err != nil {
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

	return t.validateTLSFields()
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

func (t *TLS) validateTLSFields() error {
	for protocol := range strings.FieldsSeq(t.Protocols) {
		if protocol != clusterTLSProtocol12 {
			return errValidationInvalidValue("protocols", protocol, []string{clusterTLSProtocol12})
		}
	}

	if t.CipherSuite == "" {
		return nil
	}
	found := false
	for suite := range strings.SplitSeq(t.CipherSuite, ":") {
		suite = strings.TrimSpace(suite)
		if suite == "" {
			continue
		}
		found = true
		if !clusterCipherSuites[suite] {
			return errValidationInvalidValue("cipher-suite", suite, "known Go TLS cipher suites")
		}
	}
	if !found {
		return errValidationInvalidValue("cipher-suite", t.CipherSuite, "known Go TLS cipher suites")
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
