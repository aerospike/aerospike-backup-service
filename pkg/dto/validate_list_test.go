package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateUniqueNonEmpty(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		items   []string
		wantErr string
	}{
		{
			name:  "unique values",
			field: "set-list",
			items: []string{"a", "b"},
		},
		{
			name:  "empty list",
			field: "set-list",
		},
		{
			name:    "duplicate values",
			field:   "set-list",
			items:   []string{"set1", "set1"},
			wantErr: "set-list contains duplicate value",
		},
		{
			name:    "empty value",
			field:   "bin-list",
			items:   []string{"bin1", ""},
			wantErr: `"bin-list[1]" required`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUniqueNonEmpty(tt.field, tt.items)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
