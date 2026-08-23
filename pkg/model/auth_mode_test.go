package model

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
)

func TestCredentialsAuthModeOrDefault(t *testing.T) {
	var creds *Credentials
	assert.Equal(t, *defaultConfig.credentials.AuthMode, creds.AuthModeOrDefault())

	creds = &Credentials{}
	assert.Equal(t, *defaultConfig.credentials.AuthMode, creds.AuthModeOrDefault())

	creds = &Credentials{AuthMode: ptr.Of(AuthModePKI)}
	assert.Equal(t, AuthModePKI, creds.AuthModeOrDefault())
}
