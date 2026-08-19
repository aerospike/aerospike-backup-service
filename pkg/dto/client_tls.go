package dto

import (
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/safepath"
)

// ClientTLS represents the TLS configuration options relevant for client-side connections.
// It includes paths to CA certificates, client certificates, and client keys for mutual TLS.
//
//nolint:lll
type ClientTLS struct {
	// Path to a trusted CA certificate file in PEM format.
	CAFile string `yaml:"ca-file,omitempty" json:"ca-file,omitempty" example:"/path/to/ca.pem" extensions:"x-nullable"`
	// TLSName used for server certificate verification (ServerName for SNI).
	Name string `yaml:"name,omitempty" json:"name,omitempty" example:"example.com" extensions:"x-nullable"`
	// Path to a client certificate file for mutual TLS authentication.
	Certfile string `yaml:"cert-file,omitempty" json:"cert-file,omitempty" example:"/path/to/cert.pem" extensions:"x-nullable"`
	// Path to a client private key file for mutual TLS authentication.
	Keyfile string `yaml:"key-file,omitempty" json:"key-file,omitempty" example:"/path/to/key.pem" extensions:"x-nullable"`
}

const (
	caField   = "ca-file"
	certField = "cert-file"
	keyField  = "key-file"
	nameField = "name"
)

// Validate validates the ClientTLS configuration.
//
// ca-file is optional and independent: it trusts the server certificate.
// cert-file, key-file, and name are a separate mTLS set:
// together they identify this client to the server.
func (c *ClientTLS) Validate(opts ...ValidationOption) error {
	if c == nil {
		return nil
	}

	if !slices.Contains(opts, ValidationSkipTLSFiles) {
		if err := c.validatePaths(); err != nil {
			return err
		}

		if err := c.verifyFilesExist(); err != nil {
			return err
		}
	}

	// mTLS: cert-file, key-file, and name must be set together.
	if c.Certfile != "" {
		if c.Keyfile == "" {
			return errValidationRequires(certField, keyField)
		}
		if c.Name == "" {
			return errValidationRequires(certField, nameField)
		}
	}

	if c.Keyfile != "" {
		if c.Certfile == "" {
			return errValidationRequires(keyField, certField)
		}
		if c.Name == "" {
			return errValidationRequires(keyField, nameField)
		}
	}

	if c.Name != "" {
		if c.Certfile == "" {
			return errValidationRequires(nameField, certField)
		}
		if c.Keyfile == "" {
			return errValidationRequires(nameField, keyField)
		}
	}

	return nil
}

func (c *ClientTLS) ToModel() model.ClientTLS {
	if c == nil {
		return model.ClientTLS{}
	}

	return model.ClientTLS{
		CAFile:   c.CAFile,
		Name:     c.Name,
		Certfile: c.Certfile,
		Keyfile:  c.Keyfile,
	}
}

func (c *ClientTLS) validatePaths() error {
	for field, path := range map[string]string{
		caField:   c.CAFile,
		certField: c.Certfile,
		keyField:  c.Keyfile,
	} {
		if err := safepath.ValidateClean(path); err != nil {
			return errValidationInvalidPath(field, path, err)
		}
	}

	return nil
}

func (c *ClientTLS) verifyFilesExist() error {
	for field, path := range map[string]string{
		caField:   c.CAFile,
		certField: c.Certfile,
		keyField:  c.Keyfile,
	} {
		if err := safepath.EnsureFileExists(path); err != nil {
			return errValidationNotFound(field, path)
		}
	}

	return nil
}
