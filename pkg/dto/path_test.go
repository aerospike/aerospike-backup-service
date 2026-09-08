package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPathValidate(t *testing.T) {
	tests := []struct {
		name       string
		path       Path
		wantErr    bool
		wantErrSub string
	}{
		{name: "empty path", path: ""},
		{name: "clean relative path", path: "testdata/password.txt"},
		{name: "clean absolute path", path: "/etc/ssl/certs/ca.pem"},
		{name: "trailing slash", path: "backups/", wantErr: true, wantErrSub: "must not end with '/'"},
		{name: "trailing slash absolute", path: "/etc/certs/", wantErr: true, wantErrSub: "must not end with '/'"},
		{name: "parent traversal", path: "certs/../../outside.pem", wantErr: true, wantErrSub: "must not contain '..'"},
		{name: "leading parent traversal segment", path: "../etc/passwd", wantErr: true, wantErrSub: "must not contain '..'"},
		{name: "dot prefix", path: "./certs/ca.pem", wantErr: true, wantErrSub: "canonical form"},
		{name: "redundant separators", path: "/etc//ssl/ca.pem", wantErr: true, wantErrSub: "canonical form"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.path.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrSub != "" {
					require.Contains(t, err.Error(), tt.wantErrSub)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPathValidateRelative(t *testing.T) {
	tests := []struct {
		name    string
		path    Path
		wantErr bool
	}{
		{name: "empty path", path: ""},
		{name: "relative path", path: "backups"},
		{name: "nested relative path", path: "var/backups"},
		{name: "absolute path", path: "/var/backups", wantErr: true},
		{name: "parent directory", path: "..", wantErr: true},
		{name: "traversal path", path: "../backups", wantErr: true},
		{name: "dot prefix", path: "./backups", wantErr: true},
		{name: "trailing slash", path: "backups/", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.path.ValidateRelative()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
