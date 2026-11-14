// Package router picks a backend per request.
package router

import (
	"fmt"
	"sync/atomic"

	"github.com/pavlozubrytskyi987/rpcbalancer/internal/backend"
)

// RoundRobin cycles through backends in order. Safe for concurrent use.
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

// Pick returns the next backend in rotation.
func (r *RoundRobin) Pick() *backend.Backend {
	i := r.next.Add(1) - 1
	return r.backends[int(i%uint64(len(r.backends)))]
}
