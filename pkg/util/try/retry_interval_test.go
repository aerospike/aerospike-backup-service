package try

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The back-off saturates at the largest Duration: an overflowing conversion is
// implementation-defined and wraps negative on amd64, which would be a hot retry loop.
func TestNextRetryInterval_Saturates(t *testing.T) {
	assert.Equal(t, 2*time.Second, nextRetryInterval(time.Second, 2))
	assert.Equal(t, time.Duration(math.MaxInt64), nextRetryInterval(time.Millisecond, 1e300))
	assert.Positive(t, nextRetryInterval(time.Hour, 1e12))
}

// A saturated back-off is still cut short by context cancellation.
func TestRetry_SaturatedBackoffStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	policy := models.RetryPolicy{BaseTimeout: time.Millisecond, Multiplier: 1e300, MaxRetries: 3}
	start := time.Now()
	err := Retry(ctx, policy, slog.Default(), func() error { return errors.New("boom") }, func() {})

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 5*time.Second)
}
