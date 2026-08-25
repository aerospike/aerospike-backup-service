package dto

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
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

// Validate checks that the authentication mode is supported.
func (m AuthMode) Validate() error {
	_, err := m.ToModel()
	return err
}

// ToModel converts the DTO authentication mode to the model type.
func (m AuthMode) ToModel() (*model.AuthMode, error) {
	if m == "" {
		return nil, nil
	}

	switch AuthMode(foldUpper(string(m))) {
	case AuthModeInternal:
		return ptr.Of(model.AuthModeInternal), nil
	case AuthModeExternal:
		return ptr.Of(model.AuthModeExternal), nil
	case AuthModePKI:
		return ptr.Of(model.AuthModePKI), nil
	default:
		return nil, fmt.Errorf(
			"auth-mode %q incorrect, should be one of: %s,%s,%s",
			m,
			AuthModeInternal,
			AuthModeExternal,
			AuthModePKI,
		)
	}
}

// NewAuthModeFromModel creates a DTO authentication mode from the model type.
func NewAuthModeFromModel(m *model.AuthMode) AuthMode {
	if m == nil {
		return ""
	}

	return AuthMode(*m)
}
