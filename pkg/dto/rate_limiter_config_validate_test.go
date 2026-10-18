package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
)

func TestRateLimiterConfig_Validate_RejectsMalformedWhitelistEntry(t *testing.T) {
	cfg := &RateLimiterConfig{
		WhiteList: []string{"invalid-ip"},
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "white-list contains invalid ip or cidr")
}

func TestRateLimiterConfig_Validate_AcceptsIPAndCIDRWhitelistEntries(t *testing.T) {
	cfg := &RateLimiterConfig{
		WhiteList: []string{
			"0.0.0.0",
			"127.0.0.1",
			"10.0.0.0/8",
			"0.0.0.0/0",
		},
	}

	require.NoError(t, cfg.Validate())
}

func TestRateLimiterConfig_Validate_TpsAndSize(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *RateLimiterConfig
		wantErr string
	}{
		{name: "positive", cfg: &RateLimiterConfig{Tps: ptr.Of(1), Size: ptr.Of(1)}},
		{name: "unset", cfg: &RateLimiterConfig{}},
		{name: "zero tps", cfg: &RateLimiterConfig{Tps: ptr.Of(0)}, wantErr: `"tps"`},
		{name: "negative tps", cfg: &RateLimiterConfig{Tps: ptr.Of(-1)}, wantErr: `"tps"`},
		{name: "zero size", cfg: &RateLimiterConfig{Size: ptr.Of(0)}, wantErr: `"size"`},
		{name: "negative size", cfg: &RateLimiterConfig{Size: ptr.Of(-1)}, wantErr: `"size"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, errNonPositive)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
