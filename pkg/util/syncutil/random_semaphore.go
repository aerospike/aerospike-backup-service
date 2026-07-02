package syncutil

import (
	"context"
	"math/rand/v2"
	"sync"
)

// Limiter coordinates bounded concurrent access to a shared resource.
type Limiter interface {
	// Acquire blocks until n tokens are available or ctx is canceled. When enough
	// tokens are already free, they are taken immediately without enqueueing.
	Acquire(ctx context.Context, n int64) error
	// Release returns n tokens to the semaphore and wakes waiters that can proceed.
	Release(n int64)
	// TryAcquire takes n tokens immediately. It returns false when fewer than n
	// tokens are available and does not block.
	TryAcquire(n int64) bool
}

// waiter represents a blocked Acquire call waiting for n tokens.
type waiter struct {
	n     int64
	ready chan struct{}
}

// RandomSemaphore is a counting semaphore that wakes blocked waiters in random
// order rather than FIFO. Random selection spreads wakeups across competing
// goroutines, which helps avoid convoying when many waiters queue for the same
// tokens (for example, scan parallelism slots shared across namespaces).
//
// Acquire takes available tokens immediately when s.tokens >= n, even if other
// goroutines are already waiting.
type RandomSemaphore struct {
	mu      sync.Mutex
	tokens  int64
	waiters []*waiter
}

// NewRandomSemaphore returns a semaphore with the given token capacity.
func NewRandomSemaphore(capacity int64) *RandomSemaphore {
	return &RandomSemaphore{
		tokens: capacity,
	}
}

// TryAcquire takes n tokens immediately. It returns false when fewer than n
// tokens are available and does not block.
func (s *RandomSemaphore) TryAcquire(n int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tokens >= n {
		s.tokens -= n
		return true
	}
	return false
}

// Acquire blocks until n tokens are available or ctx is canceled. When enough
// tokens are already free, they are taken immediately without enqueueing.
func (s *RandomSemaphore) Acquire(ctx context.Context, n int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	if s.tokens >= n { // Fast path: tokens are available immediately.
		s.tokens -= n
		s.mu.Unlock()
		return nil
	}

	w := &waiter{
		n:     n,
		ready: make(chan struct{}),
	}
	s.waiters = append(s.waiters, w)

	s.mu.Unlock()

	select {
	case <-ctx.Done():
		s.mu.Lock()
		select {
		case <-w.ready:
			// Selected by notify but canceled before observing ready.
			// Return the tokens and try to wake another waiter.
			s.tokens += n
			s.notify()
			s.mu.Unlock()
			return ctx.Err()
		default:
			s.removeWaiter(w)
			s.mu.Unlock()
			return ctx.Err()
		}
	case <-w.ready:
		return nil
	}
}

// Release returns n tokens to the semaphore and wakes waiters that can proceed.
func (s *RandomSemaphore) Release(n int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens += n
	s.notify()
}

// notify satisfies as many waiters as possible using the current token balance.
// Waiters are shuffled, then scanned in that order until tokens are exhausted.
// Waiters that need more tokens than remain stay queued for a later notify.
//
// Must be called with s.mu held.
func (s *RandomSemaphore) notify() {
	n := len(s.waiters)
	if n == 0 || s.tokens == 0 {
		return
	}

	rand.Shuffle(n, func(i, j int) {
		s.waiters[i], s.waiters[j] = s.waiters[j], s.waiters[i]
	})

	alive := s.waiters[:0]
	for _, w := range s.waiters[:n] {
		if s.tokens >= w.n {
			s.tokens -= w.n
			close(w.ready)
			continue
		}
		alive = append(alive, w)
	}

	for i := len(alive); i < n; i++ {
		s.waiters[i] = nil
	}
	s.waiters = alive
}

// removeWaiter deletes w from the queue. Must be called with s.mu held.
func (s *RandomSemaphore) removeWaiter(w *waiter) {
	for i := range s.waiters {
		if s.waiters[i] != w {
			continue
		}
		lastIdx := len(s.waiters) - 1
		if i != lastIdx {
			s.waiters[i] = s.waiters[lastIdx]
		}
		s.waiters[lastIdx] = nil
		s.waiters = s.waiters[:lastIdx]
		return
	}
}
