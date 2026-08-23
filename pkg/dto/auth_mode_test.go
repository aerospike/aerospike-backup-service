package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthModeToModel(t *testing.T) {
	tests := []struct {
		name    string
		value   AuthMode
		want    *model.AuthMode
		wantErr bool
	}{
		{
			name:  "empty is unset",
			value: "",
			want:  nil,
		},
		{
			name:  "internal",
			value: AuthModeInternal,
			want:  ptr.Of(model.AuthModeInternal),
		},
		{
			name:  "internal lowercase",
			value: "internal",
			want:  ptr.Of(model.AuthModeInternal),
		},
		{
			name:  "external",
			value: AuthModeExternal,
			want:  ptr.Of(model.AuthModeExternal),
		},
		{
			name:  "pki",
			value: AuthModePKI,
			want:  ptr.Of(model.AuthModePKI),
		},
		{
			name:    "invalid",
			value:   "LDAP",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.value.ToModel()
			if test.wantErr {
				require.Error(t, err)
				require.Error(t, test.value.Validate())
				return
			}
			require.NoError(t, err)
			require.NoError(t, test.value.Validate())
			assert.Equal(t, test.want, got)
			if test.value == "" || test.value == AuthModeInternal ||
				test.value == AuthModeExternal || test.value == AuthModePKI {
				assert.Equal(t, test.value, NewAuthModeFromModel(got))
			}
		})
	}
}
