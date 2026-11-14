// Package health probes backends on an interval and flips their alive bit.
package health

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/pavlozubrytskyi987/rpcbalancer/internal/backend"
)

// Options tunes a Checker. Zero values fall back to sane defaults.
type Options struct {
	Interval  time.Duration // how often to probe each backend
	Timeout   time.Duration // per-probe deadline
	DownAfter int           // N consecutive failures flips a backend to down
	UpAfter   int           // N consecutive successes flips it back to up
}

func (o Options) withDefaults() Options {
	if o.Interval <= 0 {
		o.Interval = 5 * time.Second
	}
	if o.Timeout <= 0 {
		o.Timeout = 2 * time.Second
	}
	if o.DownAfter <= 0 {
		o.DownAfter = 2
	}
	if o.UpAfter <= 0 {
		o.UpAfter = 2
	}
	return o
}

// Checker probes a fixed set of backends in parallel.
type Checker struct {
	backends []*backend.Backend
	client   *http.Client
	opts     Options
	log      *slog.Logger
}

func New(backends []*backend.Backend, opts Options, log *slog.Logger) *Checker {
	opts = opts.withDefaults()
	return &Checker{
		backends: backends,
		client:   &http.Client{Timeout: opts.Timeout},
		opts:     opts,
		log:      log,
	}
}

// Run spawns one goroutine per backend and blocks until ctx is cancelled.
// On return all probe goroutines have exited.
func (c *Checker) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, b := range c.backends {
		wg.Go(func() { c.loop(ctx, b) })
	}
	wg.Wait()
}

func (c *Checker) loop(ctx context.Context, b *backend.Backend) {
	var fails, wins int

	t := time.NewTicker(c.opts.Interval)
	defer t.Stop()

	for {
		// probe first, so we don't wait a whole interval before the first check
		if c.probe(ctx, b) {
			fails = 0
			wins++
			if !b.Alive() && wins >= c.opts.UpAfter {
				b.SetAlive(true)
				c.log.Info("backend up", "name", b.Name)
			}
		} else {
			wins = 0
			fails++
			if b.Alive() && fails >= c.opts.DownAfter {
				b.SetAlive(false)
				c.log.Warn("backend down", "name", b.Name)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// probe is healthy on any response under 500. 4xx counts as alive because
// the server answered; we just asked it a question it does not accept.
// A transport error or 5xx counts as down.
func (c *Checker) probe(ctx context.Context, b *backend.Backend) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.Target.String(), nil)
	if err != nil {
		return false
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}
