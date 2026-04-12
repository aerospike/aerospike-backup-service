package syncutil

import (
	"context"
	"math/rand/v2"
	"time"

	"golang.org/x/sync/semaphore"
)

// DualLimiter coordinates acquisition across two semaphores.
// It ensures that both are acquired atomically before proceeding,
// which is useful for enforcing both a global limit and a per-routine limit.
// Either semaphore can be nil, in which case only the non-nil one is used.
type DualLimiter struct {
	l1 *semaphore.Weighted
	l2 *semaphore.Weighted
}

// NewDualLimiter creates a new DualLimiter that requires acquiring from both
// l1 and l2 before proceeding. Either can be nil.
func NewDualLimiter(l1, l2 *semaphore.Weighted) *DualLimiter {
	return &DualLimiter{l1: l1, l2: l2}
}

// Acquire implements alternating blocking acquisition with a randomized jitter
// to prevent livelocks when multiple goroutines compete for the same two resources.
func (d *DualLimiter) Acquire(ctx context.Context, n int64) error {
	if d.l1 == nil && d.l2 == nil {
		return nil
	}
	if d.l1 == nil {
		return d.l2.Acquire(ctx, n)
	}
	if d.l2 == nil {
		return d.l1.Acquire(ctx, n)
	}

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

// Release releases n units from both underlying semaphores.
func (d *DualLimiter) Release(n int64) {
	if d.l1 != nil {
		d.l1.Release(n)
	}
	if d.l2 != nil {
		d.l2.Release(n)
	}
}

// TryAcquire attempts to acquire n units from both semaphores immediately.
func (d *DualLimiter) TryAcquire(n int64) bool {
	if d.l1 == nil && d.l2 == nil {
		return true
	}
	if d.l1 == nil {
		return d.l2.TryAcquire(n)
	}
	if d.l2 == nil {
		return d.l1.TryAcquire(n)
	}

	if !d.l1.TryAcquire(n) {
		return false
	}
	if !d.l2.TryAcquire(n) {
		d.l1.Release(n)

		return false
	}

	return true
}
