package sync

import (
	"context"
	"math/rand/v2"
	"time"
)

// limiter limits concurrent access (e.g. number of parallel scans).
// *semaphore.Weighted from [golang.org/x/sync/semaphore] implements this interface.
type limiter interface {
	// Acquire acquires the semaphore with a weight of n, blocking until resources
	// are available or ctx is done. On success, returns nil. On failure, returns
	// ctx.Err() and leaves the semaphore unchanged.
	Acquire(ctx context.Context, n int64) error
	// TryAcquire acquires the semaphore with a weight of n without blocking.
	// On success, returns true. On failure, returns false and leaves the semaphore unchanged.
	TryAcquire(n int64) bool
	// Release releases the semaphore with a weight of n.
	Release(n int64)
}

// DualLimiter coordinates acquisition across two underlying limiters.
type DualLimiter struct {
	l1 limiter
	l2 limiter
}

// NewDualLimiter creates a new DualLimiter.
func NewDualLimiter(l1, l2 limiter) *DualLimiter {
	return &DualLimiter{l1: l1, l2: l2}
}

// Acquire implements alternating blocking acquisition with a randomized jitter
// to prevent livelocks when multiple goroutines compete for the same two resources.
func (d *DualLimiter) Acquire(ctx context.Context, n int64) error {
	primary, secondary := d.l1, d.l2

	for {
		// 1. Block on the primary limiter
		if err := primary.Acquire(ctx, n); err != nil {
			return err
		}

		// 2. Try to grab the secondary (non-blocking)
		if secondary.TryAcquire(n) {
			return nil // Success!
		}

		// 3. Contention detected. Release the first one to avoid deadlock.
		primary.Release(n)

		// 4. Randomized sleep + Context check.
		// This breaks the synchronization that causes Livelock.
		// #nosec G404
		jitter := time.Duration(rand.IntN(10)+1) * time.Millisecond

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jitter):
			// Time to try again
		}

		// 5. Flip-flop so we don't keep hammering the same resource first.
		primary, secondary = secondary, primary
	}
}

// Release releases n units from both underlying limiters.
func (d *DualLimiter) Release(n int64) {
	d.l1.Release(n)
	d.l2.Release(n)
}

// TryAcquire attempts to acquire n units from both limiters immediately.
func (d *DualLimiter) TryAcquire(n int64) bool {
	if !d.l1.TryAcquire(n) {
		return false
	}
	if !d.l2.TryAcquire(n) {
		d.l1.Release(n)
		return false
	}
	return true
}
