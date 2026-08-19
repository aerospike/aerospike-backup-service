package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncryptionPolicy_Validate_Success(t *testing.T) {
	tests := map[string]EncryptionPolicy{
		"NONE without keys": {
			Mode: EncryptNone,
		},
		"AES128 with key file": {
			Mode:    EncryptAES128,
			KeyFile: "path",
		},
		"AES256 with key env": {
			Mode:   EncryptAES256,
			KeyEnv: "path",
		},
		"AES256 with key secret": {
			Mode:      EncryptAES256,
			KeySecret: "path",
		},
	}

	for name, policy := range tests {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, policy.Validate(ValidationDefault))
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
				KeyFile: "path",
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
				KeyEnv:    "path",
				KeySecret: "path",
			},
			wantIsErr: errMutuallyExclusive,
		},
		"AES256 with three keys set": {
			policy: EncryptionPolicy{
				Mode:      EncryptAES256,
				KeyEnv:    "path",
				KeyFile:   "path",
				KeySecret: "path",
			},
			wantIsErr: errMutuallyExclusive,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.policy.Validate(ValidationDefault)
			require.ErrorIs(t, err, tc.wantIsErr)
		})
	}
}
