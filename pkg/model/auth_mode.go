package model

import (
	"fmt"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// AuthMode identifies the Aerospike cluster authentication mode.
type AuthMode string

const (
	// AuthModeInternal is the default Aerospike internal authentication mode.
	AuthModeInternal AuthMode = "INTERNAL"
	// AuthModeExternal is the Aerospike external (e.g. LDAP) authentication mode.
	AuthModeExternal AuthMode = "EXTERNAL"
	// AuthModePKI is the Aerospike PKI (TLS certificate) authentication mode.
	AuthModePKI AuthMode = "PKI"
)

// String returns a string representation of AuthMode. A nil receiver yields empty string.
func (m *AuthMode) String() string {
	if m == nil {
		return ""
	}

	return string(*m)
}

// ParseAuthMode parses a configured auth-mode value.
// An empty value is valid and yields a nil auth mode.
func ParseAuthMode(value string) (*AuthMode, error) {
	if value == "" {
		return nil, nil
	}

	switch strings.ToUpper(strings.TrimSpace(value)) {
	case string(AuthModeInternal):
		return ptr.Of(AuthModeInternal), nil
	case string(AuthModeExternal):
		return ptr.Of(AuthModeExternal), nil
	case string(AuthModePKI):
		return ptr.Of(AuthModePKI), nil
	default:
		return nil, fmt.Errorf(
			"auth-mode %q incorrect, should be one of: %s,%s,%s",
			value,
			AuthModeInternal,
			AuthModeExternal,
			AuthModePKI,
		)
	}
}
