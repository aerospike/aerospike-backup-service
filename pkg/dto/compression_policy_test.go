package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompressionModeValidate(t *testing.T) {
	tests := []struct {
		name    string
		mode    CompressionMode
		wantErr bool
	}{
		{
			name: "none",
			mode: CompressionModeNone,
		},
		{
			name: "zstd",
			mode: CompressionModeZSTD,
		},
		{
			name: "empty",
			mode: "",
		},
		{
			name: "lowercase",
			mode: "zstd",
		},
		{
			name:    "unsupported",
			mode:    "GZIP",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.mode.Validate()
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCompressionModeModelConversion(t *testing.T) {
	assert.Equal(t, model.CompressionModeZSTD, CompressionModeZSTD.ToModel())
	assert.Equal(t, model.CompressionModeZSTD, CompressionMode("zstd").ToModel())
	assert.Equal(t, CompressionModeZSTD, NewCompressionModeFromModel(model.CompressionModeZSTD))
}

func TestCompressionPolicy_Validate_Success(t *testing.T) {
	tests := map[string]CompressionPolicy{
		"NONE without level": {
			Mode: CompressionModeNone,
		},
		"lowercase none without level": {
			Mode: "none",
		},
		"ZSTD with level": {
			Mode:  CompressionModeZSTD,
			Level: ptr.Of[int32](5),
		},
		"ZSTD with explicit zero level": {
			Mode:  CompressionModeZSTD,
			Level: ptr.Of[int32](0),
		},
		"ZSTD with fastest level": {
			Mode:  CompressionModeZSTD,
			Level: ptr.Of[int32](-1),
		},
		"ZSTD with best level": {
			Mode:  CompressionModeZSTD,
			Level: ptr.Of[int32](22),
		},
	}

	for name, policy := range tests {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, policy.Validate())
		})
	}
}

func TestCompressionPolicy_Validate_Invalid(t *testing.T) {
	type testCase struct {
		policy    CompressionPolicy
		wantIsErr error
	}

	tests := map[string]testCase{
		"empty mode": {
			policy:    CompressionPolicy{},
			wantIsErr: errEmpty,
		},
		"empty mode with level": {
			policy:    CompressionPolicy{Level: ptr.Of[int32](5)},
			wantIsErr: errEmpty,
		},
		"whitespace mode": {
			policy:    CompressionPolicy{Mode: "  "},
			wantIsErr: errEmpty,
		},
		"invalid mode value": {
			policy:    CompressionPolicy{Mode: "GZIP"},
			wantIsErr: errInvalidValue,
		},
		"NONE with a level set": {
			policy:    CompressionPolicy{Mode: CompressionModeNone, Level: ptr.Of[int32](5)},
			wantIsErr: errMutuallyExclusive,
		},
		"NONE with an explicit zero level": {
			policy:    CompressionPolicy{Mode: CompressionModeNone, Level: ptr.Of[int32](0)},
			wantIsErr: errMutuallyExclusive,
		},
		"ZSTD without level": {
			policy:    CompressionPolicy{Mode: CompressionModeZSTD},
			wantIsErr: errMissingDependency,
		},
		"ZSTD with level below range": {
			policy:    CompressionPolicy{Mode: CompressionModeZSTD, Level: ptr.Of[int32](-2)},
			wantIsErr: errInvalidValue,
		},
		"ZSTD with level above range": {
			policy:    CompressionPolicy{Mode: CompressionModeZSTD, Level: ptr.Of[int32](23)},
			wantIsErr: errInvalidValue,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.policy.Validate()
			require.ErrorIs(t, err, tc.wantIsErr)
		})
	}
}

func TestRestoreCompressionPolicy_Validate(t *testing.T) {
	tests := map[string]struct {
		policy    RestoreCompressionPolicy
		wantIsErr error
	}{
		"NONE":               {policy: RestoreCompressionPolicy{Mode: CompressionModeNone}},
		"ZSTD":               {policy: RestoreCompressionPolicy{Mode: CompressionModeZSTD}},
		"lowercase zstd":     {policy: RestoreCompressionPolicy{Mode: "zstd"}},
		"empty mode":         {policy: RestoreCompressionPolicy{}, wantIsErr: errEmpty},
		"whitespace mode":    {policy: RestoreCompressionPolicy{Mode: "  "}, wantIsErr: errEmpty},
		"invalid mode value": {policy: RestoreCompressionPolicy{Mode: "GZIP"}, wantIsErr: errInvalidValue},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.policy.Validate()
			if tc.wantIsErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.wantIsErr)
		})
	}
}

func TestCompressionPolicy_ModelRoundTrip(t *testing.T) {
	tests := map[string]*CompressionPolicy{
		"NONE":            {Mode: CompressionModeNone},
		"ZSTD":            {Mode: CompressionModeZSTD, Level: ptr.Of[int32](5)},
		"ZSTD zero level": {Mode: CompressionModeZSTD, Level: ptr.Of[int32](0)},
	}

	for name, policy := range tests {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, policy.Validate())
			roundTripped := newCompressionPolicyFromModel(policy.ToModel())
			assert.Equal(t, policy, roundTripped)
			require.NoError(t, roundTripped.Validate())
		})
	}
}
