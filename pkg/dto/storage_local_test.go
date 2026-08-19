package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalStorage_ValidatePath(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		valid bool
	}{
		{name: "relative path", path: "backups", valid: true},
		{name: "nested relative path", path: "var/backups", valid: true},
		{name: "current directory", path: ".", valid: true},
		{name: "absolute root path", path: "/var/backups", valid: true},
		{name: "parent directory", path: ".."},
		{name: "traversal path", path: "../backups"},
		{name: "embedded traversal", path: "backups/../../outside"},
		{name: "absolute path with traversal", path: "/var/backups/../outside"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&LocalStorage{Path: tt.path}).Validate(0)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
