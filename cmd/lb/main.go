package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/pavlozubrytskyi987/rpcbalancer/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to rpcbalancer YAML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	first := cfg.Backends[0]
	target, err := url.Parse(first.URL)
	if err != nil {
		log.Fatalf("parse backend %q url: %v", first.Name, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	log.Printf("rpcbalancer listening on %s, %d backend(s), forwarding to %q",
		cfg.Listen, len(cfg.Backends), first.Name)
	log.Fatal(http.ListenAndServe(cfg.Listen, proxy))
}
