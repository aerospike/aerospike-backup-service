package syncutil

import (
	"container/list"
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

// RandomSemaphore is golang.org/x/sync/semaphore.Weighted with one change:
// blocked waiters are inserted at a random position in the queue instead of
// always at the back.
type RandomSemaphore struct {
	size    int64
	cur     int64
	mu      sync.Mutex
	waiters list.List
}

type waiter struct {
	n     int64
	ready chan<- struct{}
}

// NewRandomSemaphore returns a semaphore with the given token capacity.
func NewRandomSemaphore(n int64) *RandomSemaphore {
	return &RandomSemaphore{size: n}
}

// TryAcquire matches semaphore.Weighted.TryAcquire.
// It returns false when fewer than n tokens are available and does not block.
func (s *RandomSemaphore) TryAcquire(n int64) bool {
	s.mu.Lock()
	success := s.size-s.cur >= n && s.waiters.Len() == 0
	if success {
		s.cur += n
	}
	s.mu.Unlock()
	return success
}

// Acquire matches semaphore.Weighted.Acquire except waiters are enqueued randomly.
// Blocks until n tokens are available or ctx is canceled. When enough
// capacity is already free, it is taken immediately without enqueueing.
func (s *RandomSemaphore) Acquire(ctx context.Context, n int64) error {
	done := ctx.Done()

	s.mu.Lock()
	select {
	case <-done:
		s.mu.Unlock()
		return ctx.Err()
	default:
	}
	if s.size-s.cur >= n && s.waiters.Len() == 0 {
		s.cur += n
		s.mu.Unlock()
		return nil
	}

	if n > s.size {
		s.mu.Unlock()
		<-done
		return ctx.Err()
	}

	ready := make(chan struct{})
	w := waiter{n: n, ready: ready}
	elem := s.insertWaiter(w)
	s.mu.Unlock()

	select {
	case <-done:
		s.mu.Lock()
		select {
		case <-ready:
			s.cur -= n
			s.notifyWaiters()
		default:
			isFront := s.waiters.Front() == elem
			s.waiters.Remove(elem)
			if isFront && s.size > s.cur {
				s.notifyWaiters()
			}
		}
		s.mu.Unlock()

		return ctx.Err()

	case <-ready:
		select {
		case <-done:
			s.Release(n)
			return ctx.Err()
		default:
		}
		return nil
	}
}

// Release matches semaphore.Weighted.Release.
func (s *RandomSemaphore) Release(n int64) {
	s.mu.Lock()
	s.cur -= n
	if s.cur < 0 {
		s.mu.Unlock()
		panic("syncutil: released more than held")
	}
	s.notifyWaiters()
	s.mu.Unlock()
}

func (s *RandomSemaphore) notifyWaiters() {
	for {
		next := s.waiters.Front()
		if next == nil {
			break
		}

		w := next.Value.(waiter)
		if s.size-s.cur < w.n {
			break
		}

		s.cur += w.n
		s.waiters.Remove(next)
		close(w.ready)
	}
}

// insertWaiter adds w at a random position in the waiter queue.
func (s *RandomSemaphore) insertWaiter(w waiter) *list.Element {
	n := s.waiters.Len()
	if n == 0 {
		return s.waiters.PushBack(w)
	}

	pos := rand.IntN(n + 1) // #nosec G404 — random queue position, not security
	if pos == n {
		return s.waiters.PushBack(w)
	}

	at := s.waiters.Front()
	for range pos {
		at = at.Next()
	}

	return s.waiters.InsertBefore(w, at)
}
