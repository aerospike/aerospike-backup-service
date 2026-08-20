package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientTLS_ValidatePaths(t *testing.T) {
	tests := []struct {
		name    string
		tls     ClientTLS
		wantErr bool
	}{
		{
			name: "clean paths",
			tls: ClientTLS{
				CAFile:   "/etc/ssl/certs/ca.pem",
				Certfile: "/etc/ssl/certs/client.pem",
				Keyfile:  "/etc/ssl/private/client-key.pem",
			},
		},
		{
			name: "leading parent traversal in ca-file",
			tls: ClientTLS{
				CAFile: "../etc/passwd",
			},
			wantErr: true,
		},
		{
			name: "embedded traversal in ca-file",
			tls: ClientTLS{
				CAFile: "certs/../../outside.pem",
			},
			wantErr: true,
		},
		{
			name: "dot prefix in key-file",
			tls: ClientTLS{
				Keyfile: "./keys/client-key.pem",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tls.validatePaths()
			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, errInvalidPath)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCredentials_ValidatePasswordPath(t *testing.T) {
	//nolint:gosec // test fixtures use fake credential fields
	tests := []struct {
		name    string
		creds   Credentials
		wantErr bool
	}{
		{
			name: "clean password path",
			creds: Credentials{
				User:         "admin",
				PasswordPath: "secrets/password.txt",
			},
		},
		{
			name: "traversal password path",
			creds: Credentials{
				User:         "admin",
				PasswordPath: "secrets/../../outside/secret.txt",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.creds.Validate(ValidationDefault)
			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, errInvalidPath)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
