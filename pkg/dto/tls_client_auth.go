package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// TLSClientAuth is HTTPS client-certificate authentication.
// @Description TLSClientAuth is HTTPS client-certificate authentication.
type TLSClientAuth string

const (
	// TLSClientAuthNone does not request client certificates.
	TLSClientAuthNone TLSClientAuth = "none"
	// TLSClientAuthRequest requests a client certificate but does not require or verify it.
	TLSClientAuthRequest TLSClientAuth = "request"
	// TLSClientAuthRequireAndVerify requires and verifies a client certificate.
	TLSClientAuthRequireAndVerify TLSClientAuth = "require-and-verify"
)

var tlsClientAuthModes = []TLSClientAuth{
	TLSClientAuthNone,
	TLSClientAuthRequest,
	TLSClientAuthRequireAndVerify,
}

// Validate checks that the client authentication mode is supported.
func (a TLSClientAuth) Validate() error {
	if _, ok := canonicalEnum(a, tlsClientAuthModes); ok {
		return nil
	}

	return errValidationInvalidValue("client-auth", a, tlsClientAuthModes)
}

// ToModel converts the DTO client authentication mode to the model type.
func (a TLSClientAuth) ToModel() model.TLSClientAuth {
	c, _ := canonicalEnum(a, tlsClientAuthModes)
	return model.TLSClientAuth(c)
}

// NewTLSClientAuthFromModel creates a DTO client authentication mode from the model type.
func NewTLSClientAuthFromModel(m model.TLSClientAuth) TLSClientAuth {
	return TLSClientAuth(m)
}
