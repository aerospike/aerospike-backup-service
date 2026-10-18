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
				CAFile:   Path("/etc/ssl/certs/ca.pem"),
				Name:     "tls-name",
				Certfile: Path("/etc/ssl/certs/client.pem"),
				Keyfile:  Path("/etc/ssl/private/client-key.pem"),
			},
		},
		{
			name: "leading parent traversal in ca-file",
			tls: ClientTLS{
				CAFile: Path("../etc/passwd"),
			},
			wantErr: true,
		},
		{
			name: "embedded traversal in ca-file",
			tls: ClientTLS{
				CAFile: Path("certs/../../outside.pem"),
			},
			wantErr: true,
		},
		{
			name: "dot prefix in key-file",
			tls: ClientTLS{
				Keyfile: Path("./keys/client-key.pem"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tls.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCredentials_ValidatePasswordPath(t *testing.T) {
	tests := []struct {
		name    string
		creds   Credentials
		wantErr bool
	}{
		{
			name: "clean password path",
			creds: Credentials{
				User:         "admin",
				PasswordPath: Path("secrets/password.txt"),
			},
		},
		{
			name: "traversal password path",
			creds: Credentials{
				User:         "admin",
				PasswordPath: Path("secrets/../../outside/secret.txt"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.creds.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
