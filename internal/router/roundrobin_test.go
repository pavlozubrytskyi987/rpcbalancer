package router

import (
	"strings"
	"sync"
	"testing"

	"github.com/pavlozubrytskyi987/rpcbalancer/internal/backend"
)

func mustBackend(t *testing.T, name, url string) *backend.Backend {
	t.Helper()
	b, err := backend.New(name, url)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestNewRoundRobin_emptyRejected(t *testing.T) {
	_, err := NewRoundRobin(nil)
	if err == nil || !strings.Contains(err.Error(), "at least one backend") {
		t.Fatalf("got err = %v, want error about empty backends", err)
	}
}

func TestRoundRobin_evenDistribution(t *testing.T) {
	bs := []*backend.Backend{
		mustBackend(t, "a", "http://a:1"),
		mustBackend(t, "b", "http://b:2"),
		mustBackend(t, "c", "http://c:3"),
	}
	rr, err := NewRoundRobin(bs)
	if err != nil {
		t.Fatal(err)
	}

	const picks = 3000
	counts := map[string]int{}
	for range picks {
		counts[rr.Pick().Name]++
	}
	for _, name := range []string{"a", "b", "c"} {
		if counts[name] != picks/3 {
			t.Errorf("%s got %d picks, want %d", name, counts[name], picks/3)
		}
	}
}

func TestRoundRobin_orderIsStable(t *testing.T) {
	bs := []*backend.Backend{
		mustBackend(t, "a", "http://a:1"),
		mustBackend(t, "b", "http://b:2"),
	}
	rr, err := NewRoundRobin(bs)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "a", "b", "a"}
	for i, w := range want {
		if got := rr.Pick().Name; got != w {
			t.Errorf("pick %d = %q, want %q", i, got, w)
		}
	}
}

// TestRoundRobin_concurrent runs many goroutines picking in parallel.
// Run with `go test -race ./internal/router` to catch unsynchronized counter bugs.
func TestRoundRobin_concurrent(t *testing.T) {
	bs := []*backend.Backend{
		mustBackend(t, "a", "http://a:1"),
		mustBackend(t, "b", "http://b:2"),
		mustBackend(t, "c", "http://c:3"),
		mustBackend(t, "d", "http://d:4"),
	}
	rr, err := NewRoundRobin(bs)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 100
	const picksPerWorker = 1000

	var wg sync.WaitGroup
	var mu sync.Mutex
	counts := map[string]int{}

	for range workers {
		wg.Go(func() {
			local := map[string]int{}
			for range picksPerWorker {
				local[rr.Pick().Name]++
			}
			mu.Lock()
			for k, v := range local {
				counts[k] += v
			}
			mu.Unlock()
		})
	}
	wg.Wait()

	total := workers * picksPerWorker
	for _, name := range []string{"a", "b", "c", "d"} {
		if counts[name] != total/len(bs) {
			t.Errorf("%s got %d picks, want %d", name, counts[name], total/len(bs))
		}
	}
}
