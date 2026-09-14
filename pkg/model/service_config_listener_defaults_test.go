package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// When only service.https is configured (here with mTLS), the plaintext HTTP listener must not
// come up by default: the operator asked for a TLS-only service.
func TestServiceConfig_HTTPListenerDisabledByDefaultWithHTTPSOnly(t *testing.T) {
	t.Skip("BKRS-413: the HTTP listener defaults to enabled on 0.0.0.0:8080 with an HTTPS-only configuration")

	svc := ServiceConfig{
		ServerHTTPS: &ServerConfigHTTPS{
			CertFile:     "/certs/server.pem",
			KeyFile:      "/certs/server-key.pem",
			ClientCAFile: "/certs/client-ca.pem",
			ClientAuth:   TLSClientAuthRequireAndVerify,
		},
	}

	assert.True(t, svc.GetServerHTTPOrDefault().Disabled)
	assert.False(t, svc.GetServerHTTPSOrDefault().Disabled)
}
