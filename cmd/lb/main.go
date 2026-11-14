package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/pavlozubrytskyi987/rpcbalancer/internal/backend"
	"github.com/pavlozubrytskyi987/rpcbalancer/internal/config"
	"github.com/pavlozubrytskyi987/rpcbalancer/internal/router"
)

// picker is what main wants from a router: one backend per call.
type picker interface {
	Pick() *backend.Backend
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to rpcbalancer YAML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	backends := make([]*backend.Backend, 0, len(cfg.Backends))
	for _, bc := range cfg.Backends {
		b, err := backend.New(bc.Name, bc.URL)
		if err != nil {
			log.Fatal(err)
		}
		backends = append(backends, b)
	}

	rr, err := router.NewRoundRobin(backends)
	if err != nil {
		log.Fatal(err)
	}

	handler := newHandler(rr)

	log.Printf("rpcbalancer listening on %s, %d backend(s)", cfg.Listen, len(backends))
	log.Fatal(http.ListenAndServe(cfg.Listen, handler))
}

// newHandler picks a backend per request and forwards.
func newHandler(p picker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := p.Pick()
		log.Printf("req %s %s -> %s", r.Method, r.URL.Path, b.Name)
		b.Proxy.ServeHTTP(w, r)
	})
}
