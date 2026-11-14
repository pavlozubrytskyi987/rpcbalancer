// Package router picks a backend per request.
package router

import (
	"fmt"
	"sync/atomic"

	"github.com/pavlozubrytskyi987/rpcbalancer/internal/backend"
)

// RoundRobin cycles through backends in order, skipping dead ones.
// Safe for concurrent use.
type RoundRobin struct {
	backends []*backend.Backend
	next     atomic.Uint64
}

// NewRoundRobin needs at least one backend.
func NewRoundRobin(backends []*backend.Backend) (*RoundRobin, error) {
	if len(backends) == 0 {
		return nil, fmt.Errorf("round robin requires at least one backend")
	}
	return &RoundRobin{backends: backends}, nil
}

// Pick returns the next alive backend, or nil if every backend is down.
func (r *RoundRobin) Pick() *backend.Backend {
	n := uint64(len(r.backends))
	start := r.next.Add(1) - 1

	// scan up to n positions from start; stop at the first alive one
	for i := uint64(0); i < n; i++ {
		b := r.backends[int((start+i)%n)]
		if b.Alive() {
			return b
		}
	}
	return nil
}
