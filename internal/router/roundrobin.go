// Package router picks a backend for each incoming request.
package router

import (
	"fmt"
	"sync/atomic"

	"github.com/pavlozubrytskyi987/rpcbalancer/internal/backend"
)

// RoundRobin picks backends in order, wrapping around. Safe for concurrent use.
type RoundRobin struct {
	backends []*backend.Backend
	next     atomic.Uint64
}

// NewRoundRobin constructs a RoundRobin over the given backends.
// Returns an error if the list is empty.
func NewRoundRobin(backends []*backend.Backend) (*RoundRobin, error) {
	if len(backends) == 0 {
		return nil, fmt.Errorf("round robin requires at least one backend")
	}
	return &RoundRobin{backends: backends}, nil
}

// Pick returns the next backend in rotation. Safe to call from many goroutines.
func (r *RoundRobin) Pick() *backend.Backend {
	i := r.next.Add(1) - 1
	return r.backends[int(i%uint64(len(r.backends)))]
}
