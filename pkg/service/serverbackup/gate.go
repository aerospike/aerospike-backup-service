package serverbackup

import (
	"errors"
	"sync"
)

var errAlreadyRunning = errors.New("server backup already running on cluster")

// Gate enforces a single active server-side backup per source cluster.
type Gate interface {
	// TryAcquire reserves the cluster for a server backup, or returns errAlreadyRunning.
	TryAcquire(clusterHash string) (release func(), err error)
	// IsActive reports whether a server backup is in progress on the cluster.
	IsActive(clusterHash string) bool
}

type gateImpl struct {
	mu     sync.Mutex
	active map[string]int
}

var _ Gate = (*gateImpl)(nil)

// NewGate returns an in-memory per-cluster server backup gate.
func NewGate() Gate {
	return &gateImpl{
		active: make(map[string]int),
	}
}

func (g *gateImpl) TryAcquire(clusterHash string) (func(), error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.active[clusterHash] > 0 {
		return nil, errAlreadyRunning
	}

	g.active[clusterHash] = 1

	return func() {
		g.release(clusterHash)
	}, nil
}

func (g *gateImpl) IsActive(clusterHash string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.active[clusterHash] > 0
}

func (g *gateImpl) release(clusterHash string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	count := g.active[clusterHash]
	if count <= 1 {
		delete(g.active, clusterHash)
		return
	}

	g.active[clusterHash] = count - 1
}
