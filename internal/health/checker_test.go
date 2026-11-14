package health

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pavlozubrytskyi987/rpcbalancer/internal/backend"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// shortOpts gives fast iterations so tests finish in under a second.
func shortOpts() Options {
	return Options{
		Interval:  10 * time.Millisecond,
		Timeout:   50 * time.Millisecond,
		DownAfter: 2,
		UpAfter:   2,
	}
}

// waitFor polls until cond is true or the deadline passes.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

func TestChecker_flipsDownThenUp(t *testing.T) {
	var fail atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "nope", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	b, err := backend.New("x", srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		New([]*backend.Backend{b}, shortOpts(), quietLogger()).Run(ctx)
		close(done)
	}()

	// start healthy, flip server to 500, expect backend to go down
	fail.Store(true)
	if !waitFor(t, 500*time.Millisecond, func() bool { return !b.Alive() }) {
		t.Fatal("backend never flipped to down")
	}

	// recover and expect it to flip back up
	fail.Store(false)
	if !waitFor(t, 500*time.Millisecond, func() bool { return b.Alive() }) {
		t.Fatal("backend never flipped back to up")
	}

	cancel()
	<-done
}

func TestChecker_deadServerGoesDown(t *testing.T) {
	// listener closed immediately so dial fails
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	b, err := backend.New("x", url)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		New([]*backend.Backend{b}, shortOpts(), quietLogger()).Run(ctx)
		close(done)
	}()

	if !waitFor(t, 500*time.Millisecond, func() bool { return !b.Alive() }) {
		t.Fatal("backend with no listener should go down")
	}

	cancel()
	<-done
}

func TestChecker_ctxCancelStopsAllGoroutines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	bs := make([]*backend.Backend, 3)
	for i := range bs {
		b, err := backend.New("b", srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		bs[i] = b
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		New(bs, shortOpts(), quietLogger()).Run(ctx)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond) // let it tick a few times
	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not return after ctx cancel")
	}
}

func TestOptions_withDefaults(t *testing.T) {
	got := Options{}.withDefaults()
	if got.Interval == 0 || got.Timeout == 0 || got.DownAfter == 0 || got.UpAfter == 0 {
		t.Errorf("defaults left a zero: %+v", got)
	}
}
