package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPathValidate(t *testing.T) {
	tests := []struct {
		name       string
		path       Path
		opts       ValidationOptions
		wantErr    bool
		wantErrSub string
	}{
		// Emptiness is governed by ValidationAllowEmpty.
		{name: "empty path required", path: "", wantErr: true},
		{name: "empty path allowed", path: "", opts: ValidationAllowEmpty},

		// Absoluteness is governed by ValidationAllowAbsolutePath.
		{name: "relative path", path: "backups"},
		{name: "nested relative path", path: "var/backups"},
		{name: "absolute path rejected by default", path: "/var/backups", wantErr: true, wantErrSub: "must be relative"},
		{name: "absolute path allowed", path: "/etc/ssl/certs/ca.pem", opts: ValidationAllowAbsolutePath},
		{
			name: "relative path still valid when absolute allowed", path: "testdata/password.txt",
			opts: ValidationAllowAbsolutePath,
		},

		// Traversal and canonical form are always enforced, whatever the options.
		{name: "parent traversal", path: "certs/../../outside.pem", wantErr: true, wantErrSub: "must not contain '..'"},
		{name: "leading parent traversal segment", path: "../etc/passwd", wantErr: true, wantErrSub: "must not contain '..'"},
		{name: "parent directory", path: "..", wantErr: true, wantErrSub: "must not contain '..'"},
		{
			name: "traversal not allowed even for absolute paths", path: "/etc/../root/key.pem",
			opts: ValidationAllowAbsolutePath, wantErr: true, wantErrSub: "must not contain '..'",
		},
		{name: "trailing slash", path: "backups/", wantErr: true, wantErrSub: "must not end with '/'"},
		{
			name: "trailing slash absolute", path: "/etc/certs/",
			opts: ValidationAllowAbsolutePath, wantErr: true, wantErrSub: "must not end with '/'",
		},
		{name: "dot prefix", path: "./backups", wantErr: true, wantErrSub: "canonical form"},
		{
			name: "redundant separators", path: "/etc//ssl/ca.pem",
			opts: ValidationAllowAbsolutePath, wantErr: true, wantErrSub: "canonical form",
		},

		// NUL bytes and shell-style home-directory shorthand are always rejected.
		{name: "embedded NUL byte", path: "backups/ca\x00.pem", wantErr: true, wantErrSub: "NUL byte"},
		{name: "home directory shorthand", path: "~/backups", wantErr: true, wantErrSub: "home-directory expansion"},
		{name: "bare tilde", path: "~", wantErr: true, wantErrSub: "home-directory expansion"},
		{
			name: "home directory shorthand absolute allowed", path: "~/backups",
			opts: ValidationAllowAbsolutePath, wantErr: true, wantErrSub: "home-directory expansion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.path.Validate(tt.opts)
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

// TestPathValidateOptionalLocalFile covers the option set shared by certificate,
// key and log file fields.
func TestPathValidateOptionalLocalFile(t *testing.T) {
	require.NoError(t, Path("").Validate(ValidationOptionalLocalFile))
	require.NoError(t, Path("/etc/ssl/certs/ca.pem").Validate(ValidationOptionalLocalFile))
	require.NoError(t, Path("testdata/ca.pem").Validate(ValidationOptionalLocalFile))
	require.Error(t, Path("/etc/ssl/../ca.pem").Validate(ValidationOptionalLocalFile))
}

// TestPathValidateEmptyErrorSentinel pins the error class for an empty required
// path: callers wrap it with errValidationInvalidPath, which reports it as a
// missing field (errEmpty) rather than an invalid one (errInvalidPath).
func TestPathValidateEmptyErrorSentinel(t *testing.T) {
	err := Path("").Validate(ValidationDefault)
	require.ErrorIs(t, err, errEmpty)
	require.ErrorIs(t, err, errValidation)

	wrapped := errValidationInvalidPath("backup-data-path", "", err)
	require.ErrorIs(t, wrapped, errEmpty)
	require.NotErrorIs(t, wrapped, errInvalidPath)
}
