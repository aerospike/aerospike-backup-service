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

// RandomSemaphore is a weighted semaphore adapted from golang.org/x/sync/semaphore.
// Waiters are inserted at a random end of the queue (front or back) so wakeups
// are spread across competing goroutines instead of strict FIFO convoying.
//
// Acquire takes available capacity immediately when size-cur >= n, even if other
// goroutines are already waiting.
// Everything else is copied from stdlib as is.
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

// TryAcquire takes n tokens immediately. It returns false when fewer than n
// tokens are available and does not block.
func (s *RandomSemaphore) TryAcquire(n int64) bool {
	s.mu.Lock()
	success := s.size-s.cur >= n
	if success {
		s.cur += n
	}
	s.mu.Unlock()
	return success
}

// Acquire blocks until n tokens are available or ctx is canceled. When enough
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

	if s.size-s.cur >= n {
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
	var elem *list.Element
	if rand.IntN(2) == 0 { // #nosec G404
		elem = s.waiters.PushFront(w)
	} else {
		elem = s.waiters.PushBack(w)
	}
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

// Release returns n tokens to the semaphore and wakes waiters that can proceed.
// It panics on negative n or when more is released than currently held.
func (s *RandomSemaphore) Release(n int64) {
	if n < 0 {
		panic("syncutil: negative release")
	}

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
