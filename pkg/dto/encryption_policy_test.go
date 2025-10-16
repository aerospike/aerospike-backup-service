package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
)

func TestEncryptionPolicy_Validate_Success(t *testing.T) {
	tests := map[string]EncryptionPolicy{
		"NONE without keys": {
			Mode: EncryptNone,
		},
		"AES128 with key file": {
			Mode:    EncryptAES128,
			KeyFile: ptr.Of("path"),
		},
		"AES256 with key env": {
			Mode:   EncryptAES256,
			KeyEnv: ptr.Of("path"),
		},
		"AES256 with key secret": {
			Mode:      EncryptAES256,
			KeySecret: ptr.Of("path"),
		},
	}

	for name, policy := range tests {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, policy.Validate())
		})
	}
}

func TestEncryptionPolicy_Validate_Invalid(t *testing.T) {
	type testCase struct {
		policy    EncryptionPolicy
		wantIsErr error
	}

	tests := map[string]testCase{
		"empty mode": {
			policy:    EncryptionPolicy{},
			wantIsErr: errEmpty,
		},
		"invalid mode value": {
			policy: EncryptionPolicy{
				Mode: "FOO",
			},
			wantIsErr: errInvalidValue,
		},
		"NONE with a key set": {
			policy: EncryptionPolicy{
				Mode:    EncryptNone,
				KeyFile: ptr.Of("path"),
			},
			wantIsErr: errMutuallyExclusive,
		},
		"AES128 without any key": {
			policy:    EncryptionPolicy{Mode: EncryptAES128},
			wantIsErr: errRequiredEither,
		},
		"AES256 with two keys set": {
			policy: EncryptionPolicy{
				Mode:      EncryptAES256,
				KeyEnv:    ptr.Of("path"),
				KeySecret: ptr.Of("path"),
			},
			wantIsErr: errMutuallyExclusive,
		},
		"AES256 with three keys set": {
			policy: EncryptionPolicy{
				Mode:      EncryptAES256,
				KeyEnv:    ptr.Of("path"),
				KeyFile:   ptr.Of("path"),
				KeySecret: ptr.Of("path"),
			},
			wantIsErr: errMutuallyExclusive,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := tc.policy.Validate()
			require.ErrorIs(t, err, tc.wantIsErr)
		})
	}
}
