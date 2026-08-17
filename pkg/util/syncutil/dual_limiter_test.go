package syncutil

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"
)

func TestDualLimiter_Acquire_Success(t *testing.T) {
	l1 := semaphore.NewWeighted(10)
	l2 := semaphore.NewWeighted(10)
	dl := NewDualLimiter(l1, l2)

	ctx := t.Context()
	err := dl.Acquire(ctx, 5)

	require.NoError(t, err)

	// Verify both are actually locked by checking remaining capacity
	// If we can't acquire 6, it means at least 5 are held.
	assert.False(t, l1.TryAcquire(6), "l1 should have 5 units held")
	assert.False(t, l2.TryAcquire(6), "l2 should have 5 units held")

	dl.Release(5)

	// Verify units were returned
	assert.True(t, l1.TryAcquire(10), "l1 should be fully released")
	assert.True(t, l2.TryAcquire(10), "l2 should be fully released")
}

func TestDualLimiter_TryAcquire_Atomicity(t *testing.T) {
	l1 := semaphore.NewWeighted(10)
	l2 := semaphore.NewWeighted(10)
	dl := NewDualLimiter(l1, l2)

	// Exhaust l2 externally
	require.True(t, l2.TryAcquire(10))

	// TryAcquire should fail because l2 is full
	success := dl.TryAcquire(5)
	assert.False(t, success, "TryAcquire should fail when one limiter is full")

	// CRITICAL: Ensure l1 was released even though TryAcquire failed
	assert.True(t, l1.TryAcquire(10), "l1 should have been released after partial failure")
}

func TestDualLimiter_Acquire_BlockingAndRetry(t *testing.T) {
	l1 := semaphore.NewWeighted(1)
	l2 := semaphore.NewWeighted(1)
	dl := NewDualLimiter(l1, l2)

	// 1. Manually block l2
	require.True(t, l2.TryAcquire(1))

	var wg sync.WaitGroup

	wg.Go(func() {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		err := dl.Acquire(ctx, 1)
		assert.NoError(t, err)
	})

	// Give the goroutine time to enter the loop and block/jitter
	time.Sleep(100 * time.Millisecond)

	// 2. Release l2. The "flip-flop" logic should now pick it up.
	l2.Release(1)

	wg.Wait()
}

func TestDualLimiter_Acquire_ContextCancel(t *testing.T) {
	l1 := semaphore.NewWeighted(1)
	l2 := semaphore.NewWeighted(1)
	dl := NewDualLimiter(l1, l2)

	// Block l1 so Acquire blocks immediately
	require.True(t, l1.TryAcquire(1))

	ctx, cancel := context.WithCancel(t.Context())

	errChan := make(chan error, 1)
	go func() {
		errChan <- dl.Acquire(ctx, 1)
	}()

	// Wait for goroutine to be parked, then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errChan:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(1 * time.Second):
		require.FailNow(t, "Acquire did not respect context cancellation in time")
	}
}

func TestDualLimiter_Release_IsSymmetric(t *testing.T) {
	l1 := semaphore.NewWeighted(5)
	l2 := semaphore.NewWeighted(5)
	dl := NewDualLimiter(l1, l2)

	// Acquire via the wrapper
	require.NoError(t, dl.Acquire(t.Context(), 3))

	// Release via the wrapper
	dl.Release(3)

	// Both should be at full capacity
	assert.True(t, l1.TryAcquire(5), "l1 was not fully released")
	assert.True(t, l2.TryAcquire(5), "l2 was not fully released")
}

func TestDualLimiter_Acquire_NilL1(t *testing.T) {
	l2 := semaphore.NewWeighted(5)
	dl := NewDualLimiter(nil, l2)

	err := dl.Acquire(t.Context(), 3)
	require.NoError(t, err)

	// l2 should have 3 units held
	assert.False(t, l2.TryAcquire(3), "l2 should have 3 units held")

	dl.Release(3)
	assert.True(t, l2.TryAcquire(5), "l2 should be fully released")
}

func TestDualLimiter_Acquire_NilL2(t *testing.T) {
	l1 := semaphore.NewWeighted(5)
	dl := NewDualLimiter(l1, nil)

	err := dl.Acquire(t.Context(), 3)
	require.NoError(t, err)

	// l1 should have 3 units held
	assert.False(t, l1.TryAcquire(3), "l1 should have 3 units held")

	dl.Release(3)
	assert.True(t, l1.TryAcquire(5), "l1 should be fully released")
}

func TestDualLimiter_Acquire_BothNil(t *testing.T) {
	dl := NewDualLimiter(nil, nil)

	err := dl.Acquire(t.Context(), 100)
	require.NoError(t, err)

	// Should be no-op
	dl.Release(100)
}

func TestDualLimiter_TryAcquire_NilL1(t *testing.T) {
	l2 := semaphore.NewWeighted(5)
	dl := NewDualLimiter(nil, l2)

	success := dl.TryAcquire(3)
	assert.True(t, success)

	// l2 should have 3 units held
	assert.False(t, l2.TryAcquire(3), "l2 should have 3 units held")

	dl.Release(3)
}

func TestDualLimiter_TryAcquire_NilL2(t *testing.T) {
	l1 := semaphore.NewWeighted(5)
	dl := NewDualLimiter(l1, nil)

	success := dl.TryAcquire(3)
	assert.True(t, success)

	// l1 should have 3 units held
	assert.False(t, l1.TryAcquire(3), "l1 should have 3 units held")

	dl.Release(3)
}

func TestDualLimiter_TryAcquire_BothNil(t *testing.T) {
	dl := NewDualLimiter(nil, nil)

	success := dl.TryAcquire(100)
	assert.True(t, success)

	dl.Release(100)
}
