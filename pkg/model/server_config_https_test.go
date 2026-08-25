package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServerHTTPSConfigDefaults(t *testing.T) {
	config := &ServerConfigHTTPS{}

	assert.False(t, config.GetDisabledOrDefault())
	assert.Equal(t, "0.0.0.0", config.GetAddressOrDefault())
	assert.Equal(t, Port(8443), config.GetPortOrDefault())
	assert.Equal(t, "/", config.GetContextPathOrDefault())
	assert.Equal(t, 5*time.Second, config.GetTimeoutOrDefault())
	assert.Equal(t, 30*time.Second, config.GetReadTimeoutOrDefault())
	assert.Equal(t, 60*time.Second, config.GetWriteTimeoutOrDefault())
	assert.Equal(t, 120*time.Second, config.GetIdleTimeoutOrDefault())
	assert.Equal(t, TLSMinVersion12, config.GetMinVersionOrDefault())
	assert.Nil(t, config.GetCipherSuitesOrDefault())
	assert.Equal(t, TLSClientAuthNone, config.GetClientAuthOrDefault())
	assert.NotNil(t, config.GetRateOrDefault())
}
