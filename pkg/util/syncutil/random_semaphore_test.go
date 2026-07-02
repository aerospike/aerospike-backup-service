package syncutil

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandomSemaphore_TryAcquire_SuccessAndFailure(t *testing.T) {
	s := NewRandomSemaphore(10)

	assert.True(t, s.TryAcquire(3))
	assert.False(t, s.TryAcquire(8))
	assert.True(t, s.TryAcquire(7))

	s.Release(10)
	assert.True(t, s.TryAcquire(10))
}

func TestRandomSemaphore_Acquire_FastPath(t *testing.T) {
	s := NewRandomSemaphore(5)

	err := s.Acquire(t.Context(), 3)
	require.NoError(t, err)

	assert.False(t, s.TryAcquire(3))
	assert.True(t, s.TryAcquire(2))

	s.Release(3)
}

func TestRandomSemaphore_Acquire_BypassWithWaiters(t *testing.T) {
	s := NewRandomSemaphore(0)

	acquired := make(chan struct{})
	go func() {
		err := s.Acquire(t.Context(), 10)
		assert.NoError(t, err)
		close(acquired)
	}()

	time.Sleep(50 * time.Millisecond)
	s.Release(5)
	time.Sleep(50 * time.Millisecond)

	err := s.Acquire(t.Context(), 2)
	require.NoError(t, err, "should bypass the queue when tokens are available")

	select {
	case <-acquired:
		t.Fatal("blocked waiter should not proceed until enough tokens are released")
	default:
	}

	assert.True(t, s.TryAcquire(3), "three tokens should remain after the bypass acquire")
	assert.False(t, s.TryAcquire(1))

	s.Release(10)

	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked waiter was not satisfied after enough tokens were released")
	}
}

func TestRandomSemaphore_Acquire_AlreadyCanceledContext(t *testing.T) {
	s := NewRandomSemaphore(10)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := s.Acquire(ctx, 1)
	require.ErrorIs(t, err, context.Canceled)
	assert.True(t, s.TryAcquire(10), "no tokens should have been consumed")
}

func TestRandomSemaphore_Acquire_BlockingUntilRelease(t *testing.T) {
	s := NewRandomSemaphore(1)
	require.True(t, s.TryAcquire(1))

	done := make(chan struct{})
	go func() {
		err := s.Acquire(t.Context(), 1)
		assert.NoError(t, err)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Acquire should block until tokens are released")
	case <-time.After(50 * time.Millisecond):
	}

	s.Release(1)

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Acquire did not unblock after Release")
	}
}

func TestRandomSemaphore_Acquire_ContextCancelWhileWaiting(t *testing.T) {
	s := NewRandomSemaphore(1)
	require.True(t, s.TryAcquire(1))

	ctx, cancel := context.WithCancel(t.Context())
	errChan := make(chan error, 1)

	go func() {
		errChan <- s.Acquire(ctx, 1)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errChan:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(1 * time.Second):
		t.Fatal("Acquire did not respect context cancellation")
	}

	// Token should still be held by the initial TryAcquire.
	assert.False(t, s.TryAcquire(1))
	s.Release(1)
}

func TestRandomSemaphore_Acquire_ContextCancelAfterPicked(t *testing.T) {
	// Exercise the path where a waiter is selected but the context is canceled
	// before the goroutine observes w.ready.
	const attempts = 200

	for i := range attempts {
		s := NewRandomSemaphore(0)

		ctx, cancel := context.WithCancel(t.Context())
		errChan := make(chan error, 1)

		go func() {
			errChan <- s.Acquire(ctx, 1)
		}()

		time.Sleep(time.Millisecond)
		cancel()
		s.Release(1)

		select {
		case err := <-errChan:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(1 * time.Second):
			t.Fatal("Acquire did not return after cancel and release")
		}

		// Tokens from a canceled-but-picked waiter must be returned for others.
		assert.True(t, s.TryAcquire(1), "attempt %d: token should be available after canceled pickup", i)
	}
}

func TestRandomSemaphore_Release_UnblocksMultipleWaiters(t *testing.T) {
	s := NewRandomSemaphore(0)

	const waiters = 5
	var wg sync.WaitGroup
	wg.Add(waiters)

	for range waiters {
		go func() {
			defer wg.Done()
			err := s.Acquire(t.Context(), 1)
			assert.NoError(t, err)
		}()
	}

	time.Sleep(50 * time.Millisecond)
	s.Release(waiters)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("not all waiters were unblocked")
	}
}

func TestRandomSemaphore_Acquire_DifferentRequestSizes(t *testing.T) {
	s := NewRandomSemaphore(0)

	type result struct {
		n   int64
		err error
	}

	results := make(chan result, 3)
	go func() { results <- result{n: 1, err: s.Acquire(t.Context(), 1)} }()
	go func() { results <- result{n: 2, err: s.Acquire(t.Context(), 2)} }()
	go func() { results <- result{n: 3, err: s.Acquire(t.Context(), 3)} }()

	time.Sleep(50 * time.Millisecond)
	s.Release(6)

	deadline := time.After(2 * time.Second)
	got := make(map[int64]error, 3)
	for range 3 {
		select {
		case r := <-results:
			got[r.n] = r.err
		case <-deadline:
			t.Fatal("timed out waiting for differently-sized acquires")
		}
	}

	for n, err := range got {
		require.NoError(t, err, "acquire(%d) failed", n)
	}

	assert.False(t, s.TryAcquire(1), "all released tokens should be held by waiters")
}

func TestRandomSemaphore_ConcurrentAcquireRelease(t *testing.T) {
	const (
		capacity = 8
		workers  = 32
		loops    = 100
	)

	s := NewRandomSemaphore(capacity)
	var completed atomic.Int64

	var wg sync.WaitGroup
	wg.Add(workers)

	for range workers {
		go func() {
			defer wg.Done()
			for range loops {
				err := s.Acquire(t.Context(), 1)
				if err != nil {
					t.Errorf("Acquire failed: %v", err)
					return
				}
				completed.Add(1)
				s.Release(1)
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("concurrent test timed out after %d completions", completed.Load())
	}

	assert.Equal(t, int64(workers*loops), completed.Load())
}

func TestRandomSemaphore_ImplementsLimiter(t *testing.T) {
	var _ Limiter = (*RandomSemaphore)(nil)
}
