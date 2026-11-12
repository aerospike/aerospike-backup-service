package model

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/stretchr/testify/assert"
)

func TestRetryPolicy_Getters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		policy         *RetryPolicy
		wantTimeoutMs  *int
		wantMultiplier float64
		wantMaxRetries *int
	}{
		{
			name: "every setting",
			policy: &RetryPolicy{
				BaseTimeout: optional.Of(1500 * time.Millisecond),
				Multiplier:  new(2.0),
				MaxRetries:  optional.Of(0),
			},
			wantTimeoutMs: new(1500), wantMultiplier: 2, wantMaxRetries: new(0),
		},
		{name: "no setting", policy: &RetryPolicy{}},
		{name: "no policy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantTimeoutMs, tt.policy.BaseTimeoutMsOrNil())
			assert.InDelta(t, tt.wantMultiplier, tt.policy.MultiplierOrZero(), 0)
			assert.Equal(t, tt.wantMaxRetries, tt.policy.MaxRetriesOrNil())
		})
	}
}
