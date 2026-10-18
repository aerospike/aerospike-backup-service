package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCredentials_Validate(t *testing.T) {
	tests := []struct {
		name    string
		creds   *Credentials
		wantErr bool
		errType error
	}{
		{
			name:    "nil credentials are valid",
			creds:   nil,
			wantErr: false,
		},
		{
			name:    "empty credentials are valid",
			creds:   &Credentials{},
			wantErr: false,
		},
		{
			name:    "user and password",
			creds:   &Credentials{User: "user", Password: "pass"},
			wantErr: false,
		},
		{
			name:    "user and password-path",
			creds:   &Credentials{User: "user", PasswordPath: "/path/to/password-file.txt"},
			wantErr: false,
		},
		{
			name:    "password without user",
			creds:   &Credentials{Password: "pass"},
			wantErr: true,
		},
		{
			name:    "password-path without user",
			creds:   &Credentials{PasswordPath: "/path/to/password-file.txt"},
			wantErr: true,
		},
		{
			name:    "password and password-path are mutually exclusive",
			creds:   &Credentials{User: "user", Password: "pass", PasswordPath: "/path/to/password-file.txt"},
			wantErr: true,
			errType: errMutuallyExclusive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.creds.Validate()

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAerospikeCluster_validateSeedNodesTLSConsistency(t *testing.T) {
	tests := []struct {
		name    string
		cluster *AerospikeCluster
		wantErr bool
	}{
		{
			name:    "single seed node without TLS",
			cluster: &AerospikeCluster{SeedNodes: []SeedNode{{HostName: "a", Port: 3000}}},
			wantErr: false,
		},
		{
			name: "all seed nodes with TLS",
			cluster: &AerospikeCluster{SeedNodes: []SeedNode{
				{HostName: "a", Port: 3000, TLSName: "tls-name"},
				{HostName: "b", Port: 3000, TLSName: "tls-name"},
			}},
			wantErr: false,
		},
		{
			name: "all seed nodes without TLS",
			cluster: &AerospikeCluster{SeedNodes: []SeedNode{
				{HostName: "a", Port: 3000},
				{HostName: "b", Port: 3000},
			}},
			wantErr: false,
		},
		{
			name: "mixed seed nodes with and without TLS",
			cluster: &AerospikeCluster{SeedNodes: []SeedNode{
				{HostName: "a", Port: 3000, TLSName: "tls-name"},
				{HostName: "b", Port: 3000},
			}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cluster.validateSeedNodesTLSConsistency()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
