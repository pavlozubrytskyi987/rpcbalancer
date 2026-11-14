package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pavlozubrytskyi987/rpcbalancer/internal/backend"
	"github.com/pavlozubrytskyi987/rpcbalancer/internal/config"
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
		os.Exit(1)
	}
	log.Info("bye")
}

// forward is the per-request handler: pick a backend, log it, serve via its ReverseProxy.
func forward(log *slog.Logger, p picker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := p.Pick()
		log.Info("pick", "id", middleware.RequestID(r.Context()), "backend", b.Name)
		b.Proxy.ServeHTTP(w, r)
	})
}
