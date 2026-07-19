package model

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCredentialsAuthModeOrDefault(t *testing.T) {
	var creds *Credentials
	assert.Equal(t, *defaultConfig.credentials.AuthMode, creds.AuthModeOrDefault())

	creds = &Credentials{}
	assert.Equal(t, *defaultConfig.credentials.AuthMode, creds.AuthModeOrDefault())

	creds = &Credentials{AuthMode: ptr.Of(AuthModePKI)}
	assert.Equal(t, AuthModePKI, creds.AuthModeOrDefault())
}

func TestAuthModeString(t *testing.T) {
	var unset *AuthMode
	assert.Nil(t, unset.String())
	assert.Equal(t, ptr.Of("PKI"), ptr.Of(AuthModePKI).String())
}

func TestParseAuthMode(t *testing.T) {
	tests := []struct {
		name        string
		value       *string
		expected    *AuthMode
		expectError bool
	}{
		{name: "nil", value: nil, expected: nil},
		{name: "internal", value: ptr.Of("INTERNAL"), expected: ptr.Of(AuthModeInternal)},
		{name: "internal lowercase", value: ptr.Of("internal"), expected: ptr.Of(AuthModeInternal)},
		{name: "external", value: ptr.Of("EXTERNAL"), expected: ptr.Of(AuthModeExternal)},
		{name: "pki", value: ptr.Of("PKI"), expected: ptr.Of(AuthModePKI)},
		{name: "invalid", value: ptr.Of("LDAP"), expectError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mode, err := ParseAuthMode(test.value)
			if test.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, mode)
		})
	}
}
