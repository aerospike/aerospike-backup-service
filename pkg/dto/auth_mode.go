package dto

import (
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// AuthMode is the Aerospike cluster authentication mode.
// @Description AuthMode is the Aerospike cluster authentication mode.
type AuthMode string

const (
	// AuthModeInternal is Aerospike internal authentication.
	AuthModeInternal AuthMode = "INTERNAL"
	// AuthModeExternal is Aerospike external authentication, for example LDAP.
	AuthModeExternal AuthMode = "EXTERNAL"
	// AuthModePKI is Aerospike PKI authentication.
	AuthModePKI AuthMode = "PKI"
)

var authModes = []AuthMode{AuthModeInternal, AuthModeExternal, AuthModePKI}

// Validate checks that the authentication mode is supported.
func (m AuthMode) Validate() error {
	if m == "" || slices.Contains(authModes, m) {
		return nil
	}

	return errValidationInvalidValue("auth-mode", m, authModes)
}

// ToModel converts the DTO authentication mode to the model type.
func (m AuthMode) ToModel() model.AuthMode {
	return model.AuthMode(m)
}

// NewAuthModeFromModel creates a DTO authentication mode from the model type.
func NewAuthModeFromModel(m model.AuthMode) AuthMode {
	return AuthMode(m)
}
