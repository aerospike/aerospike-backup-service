package dto

import (
	"fmt"

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

// Validate checks that the client authentication mode is supported.
func (a TLSClientAuth) Validate() error {
	_, err := a.ToModel()
	return err
}

// ToModel converts the DTO client authentication mode to the model type.
func (a TLSClientAuth) ToModel() (model.TLSClientAuth, error) {
	switch a {
	case "", TLSClientAuthNone:
		return model.TLSClientAuthNone, nil
	case TLSClientAuthRequest:
		return model.TLSClientAuthRequest, nil
	case TLSClientAuthRequireAndVerify:
		return model.TLSClientAuthRequireAndVerify, nil
	default:
		return "", fmt.Errorf("unsupported TLS client authentication mode %q", a)
	}
}

// NewTLSClientAuthFromModel creates a DTO client authentication mode from the model type.
func NewTLSClientAuthFromModel(m model.TLSClientAuth) TLSClientAuth {
	return TLSClientAuth(m)
}
