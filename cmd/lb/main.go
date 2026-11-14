package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pavlozubrytskyi987/rpcbalancer/internal/backend"
	"github.com/pavlozubrytskyi987/rpcbalancer/internal/config"
	"github.com/pavlozubrytskyi987/rpcbalancer/internal/health"
	"github.com/pavlozubrytskyi987/rpcbalancer/internal/middleware"
	"github.com/pavlozubrytskyi987/rpcbalancer/internal/router"
)

// what main wants from a router: one backend per call.
type picker interface {
	Pick() *backend.Backend
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to YAML config")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Error("load config", "err", err)
		os.Exit(1)
	}

	backends := make([]*backend.Backend, 0, len(cfg.Backends))
	for _, bc := range cfg.Backends {
		b, err := backend.New(bc.Name, bc.URL)
		if err != nil {
			log.Error("build backend", "name", bc.Name, "err", err)
			os.Exit(1)
		}
		backends = append(backends, b)
	}

	rr, err := router.NewRoundRobin(backends)
	if err != nil {
		log.Error("build router", "err", err)
		os.Exit(1)
	}

	h := middleware.WithRequestID(
		middleware.Access(log, forward(log, rr)),
	)

	srv := &http.Server{
		Addr:    cfg.Listen,
		Handler: h,

		// server-side timeouts, keep slow clients from holding sockets
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// SIGINT or SIGTERM -> ctx is done -> we start graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// health checker runs for the lifetime of the server
	ck := health.New(backends, health.Options{
		Interval:  5 * time.Second,
		Timeout:   2 * time.Second,
		DownAfter: 2,
		UpAfter:   2,
	}, log)

	var wg sync.WaitGroup
	wg.Go(func() { ck.Run(ctx) })

	go func() {
		log.Info("listening", "addr", cfg.Listen, "backends", len(backends))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server crashed", "err", err)
			stop() // tell main to exit, don't hang forever
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	// give in-flight requests a window to finish
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
	wg.Wait() // wait for the checker to exit on ctx cancel

	log.Info("bye")
}

// forward is the per-request handler: pick a backend, log it, serve via its ReverseProxy.
// If every backend is down, respond with 503.
func forward(log *slog.Logger, p picker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := p.Pick()
		if b == nil {
			log.Warn("no backend available", "id", middleware.RequestID(r.Context()))
			http.Error(w, "no backend available", http.StatusServiceUnavailable)
			return
		}
		log.Info("pick", "id", middleware.RequestID(r.Context()), "backend", b.Name)
		b.Proxy.ServeHTTP(w, r)
	})
}
