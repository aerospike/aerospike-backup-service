package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptionPolicy_ValidateCanDecrypt_Allowed(t *testing.T) {
	type testCase struct {
		policy *EncryptionPolicy
		mode   EncryptionMode
	}

	tests := map[string]testCase{
		"matching mode with key file": {
			policy: &EncryptionPolicy{Mode: EncryptionModeAES128, KeyFile: "/keys/aes.key"},
			mode:   EncryptionModeAES128,
		},
		"matching mode with key env": {
			policy: &EncryptionPolicy{Mode: EncryptionModeAES256, KeyEnv: "AES_KEY"},
			mode:   EncryptionModeAES256,
		},
		"matching mode with key secret": {
			policy: &EncryptionPolicy{Mode: EncryptionModeAES256, KeySecret: "secrets:aes"},
			mode:   EncryptionModeAES256,
		},
		"unencrypted backup needs no policy": {
			policy: nil,
			mode:   EncryptionModeNone,
		},
		"backup without a recorded mode needs no policy": {
			policy: nil,
			mode:   "",
		},
		"unencrypted backup ignores a keyless policy": {
			policy: &EncryptionPolicy{Mode: EncryptionModeAES256},
			mode:   EncryptionModeNone,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, tt.policy.ValidateCanDecrypt(tt.mode))
		})
	}
}

func TestEncryptionPolicy_ValidateCanDecrypt_Rejected(t *testing.T) {
	type testCase struct {
		policy  *EncryptionPolicy
		mode    EncryptionMode
		wantErr string
	}

	tests := map[string]testCase{
		"no policy for an encrypted backup": {
			policy:  nil,
			mode:    EncryptionModeAES128,
			wantErr: "no encryption policy was provided",
		},
		"mode mismatch": {
			policy:  &EncryptionPolicy{Mode: EncryptionModeAES256, KeyEnv: "AES_KEY"},
			mode:    EncryptionModeAES128,
			wantErr: "the provided encryption policy specifies mode 'AES256'",
		},
		"matching mode without any key": {
			policy:  &EncryptionPolicy{Mode: EncryptionModeAES128},
			mode:    EncryptionModeAES128,
			wantErr: "no encryption key (KeyFile, KeyEnv, or KeySecret) was provided",
		},
		"policy left at mode NONE": {
			policy:  &EncryptionPolicy{KeyEnv: "AES_KEY"},
			mode:    EncryptionModeAES128,
			wantErr: "the provided encryption policy specifies mode ''",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.ErrorContains(t, tt.policy.ValidateCanDecrypt(tt.mode), tt.wantErr)
		})
	}
}

func TestEncryptionMode_IsEncrypted(t *testing.T) {
	tests := map[EncryptionMode]bool{
		EncryptionModeAES128: true,
		EncryptionModeAES256: true,
		EncryptionModeNone:   false,
		"":                   false,
	}

	for mode, want := range tests {
		t.Run(string(mode), func(t *testing.T) {
			require.Equal(t, want, mode.IsEncrypted())
		})
	}
}

func TestEncryptionPolicy_ToLibraryPolicy(t *testing.T) {
	policy := &EncryptionPolicy{
		Mode:      EncryptionModeAES256,
		KeyFile:   "/keys/aes.key",
		KeyEnv:    "AES_KEY",
		KeySecret: "secrets:aes",
	}

	converted := policy.ToLibraryPolicy()

	require.NotNil(t, converted)
	assert.Equal(t, "AES256", converted.Mode)
	assert.Equal(t, "/keys/aes.key", *converted.KeyFile)
	assert.Equal(t, "AES_KEY", *converted.KeyEnv)
	assert.Equal(t, "secrets:aes", *converted.KeySecret)
}

func TestEncryptionPolicy_ToLibraryPolicy_OmitsUnsetKeys(t *testing.T) {
	policy := &EncryptionPolicy{
		Mode:   EncryptionModeAES128,
		KeyEnv: "AES_KEY",
	}

	converted := policy.ToLibraryPolicy()

	require.NotNil(t, converted)
	assert.Equal(t, "AES_KEY", *converted.KeyEnv)
	assert.Nil(t, converted.KeyFile)
	assert.Nil(t, converted.KeySecret)
}

func TestEncryptionPolicy_ToLibraryPolicy_Nil(t *testing.T) {
	var policy *EncryptionPolicy

	assert.Nil(t, policy.ToLibraryPolicy())
}
