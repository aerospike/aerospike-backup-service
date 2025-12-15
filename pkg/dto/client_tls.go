package dto

import (
	"os"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// ClientTLS represents the TLS configuration options relevant for client-side connections.
// It includes paths to CA certificates, client certificates, and client keys for mutual TLS.
//
//nolint:lll
type ClientTLS struct {
	// Path to a trusted CA certificate file in PEM format.
	CAFile *string `yaml:"ca-file,omitempty" json:"ca-file,omitempty" example:"/path/to/ca.pem" extensions:"x-nullable"`
	// TLSName used for server certificate verification (ServerName for SNI).
	Name *string `yaml:"name,omitempty" json:"name,omitempty" example:"example.com" extensions:"x-nullable"`
	// Path to a client certificate file for mutual TLS authentication.
	Certfile *string `yaml:"cert-file,omitempty" json:"cert-file,omitempty" example:"/path/to/cert.pem" extensions:"x-nullable"`
	// Path to a client private key file for mutual TLS authentication.
	Keyfile *string `yaml:"key-file,omitempty" json:"key-file,omitempty" example:"/path/to/key.pem" extensions:"x-nullable"`
}

// Validate validates the ClientTLS configuration.
func (c *ClientTLS) Validate(opts ...ValidationOption) error {
	if c == nil {
		return nil
	}

	if !slices.Contains(opts, ValidationSkipTLSFiles) {
		if err := verifyFilesExist(c); err != nil {
			return err
		}
	}

	if c.Certfile != nil {
		if c.Keyfile == nil {
			return errValidationRequires("cert-file", "key-file")
		}
		if c.Name == nil {
			return errValidationRequires("cert-file", "name")
		}
	}

	if c.Keyfile != nil {
		if c.Certfile == nil {
			return errValidationRequires("key-file", "cert-file")
		}
		if c.Name == nil {
			return errValidationRequires("key-file", "name")
		}
	}

	if c.Name != nil {
		if c.Certfile == nil {
			return errValidationRequires("name", "cert-file")
		}
		if c.Keyfile == nil {
			return errValidationRequires("name", "key-file")
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

func verifyFilesExist(c *ClientTLS) error {
	if c.CAFile != nil {
		if _, err := os.Stat(*c.CAFile); err != nil {
			return errValidationNotFound("ca-file", *c.CAFile)
		}
	}

	if c.Certfile != nil {
		if _, err := os.Stat(*c.Certfile); err != nil {
			return errValidationNotFound("cert-file", *c.Certfile)
		}
	}

	if c.Keyfile != nil {
		if _, err := os.Stat(*c.Keyfile); err != nil {
			return errValidationNotFound("key-file", *c.Keyfile)
		}
	}

	return nil
}
