package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateObjectStoragePath(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		valid bool
	}{
		{name: "empty path", path: "", valid: true},
		{name: "relative path", path: "backups", valid: true},
		{name: "nested relative path", path: "var/backups", valid: true},
		{name: "parent directory", path: ".."},
		{name: "traversal path", path: "../backups"},
		{name: "embedded traversal", path: "backups/../../outside"},
		{name: "absolute path", path: "/var/backups"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateObjectStoragePath(tt.path)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
