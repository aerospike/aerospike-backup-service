package dto

import (
	"testing"

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
