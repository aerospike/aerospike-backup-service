package model

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

// String returns the wire value of the authentication mode.
func (m AuthMode) String() string {
	return string(m)
}
