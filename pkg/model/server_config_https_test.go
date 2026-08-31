package model

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerHTTPSConfigDefaults(t *testing.T) {
	config := &ServerConfigHTTPS{}

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

func TestTLSClientAuthToTLS(t *testing.T) {
	tests := []struct {
		name    string
		value   TLSClientAuth
		want    tls.ClientAuthType
		wantErr string
	}{
		{
			name:  "empty defaults to none",
			value: "",
			want:  tls.NoClientCert,
		},
		{
			name:  "none",
			value: TLSClientAuthNone,
			want:  tls.NoClientCert,
		},
		{
			name:  "request",
			value: TLSClientAuthRequest,
			want:  tls.RequestClientCert,
		},
		{
			name:  "require-and-verify",
			value: TLSClientAuthRequireAndVerify,
			want:  tls.RequireAndVerifyClientCert,
		},
		{
			name:    "invalid",
			value:   "verify-if-given",
			wantErr: "unsupported TLS client authentication mode",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.value.ToTLS()
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}
