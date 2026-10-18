package model

import as "github.com/aerospike/aerospike-client-go/v8"

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

func (m AuthMode) ResolveAuth() as.AuthMode {
	switch m {
	case AuthModeInternal:
		return as.AuthModeInternal
	case AuthModeExternal:
		return as.AuthModeExternal
	case AuthModePKI:
		return as.AuthModePKI
	default:
		return as.AuthModeInternal
	}
}
