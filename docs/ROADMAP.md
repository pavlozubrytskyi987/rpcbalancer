# roadmap

## architecture

```
            ┌──────────────────────────────────────────┐
            │ middleware: request id, log, metrics,    │
 client ──► │             rate limit                   │
            ├──────────────────────────────────────────┤
            │ proxy handler                            │
            │   read body (size capped)                │
            │   parse json-rpc envelope                │
            │   apply method policy                    │
            │   cache lookup                           │
            │   pick → forward → retry/hedge/broadcast │
            │   record latency + outcome               │
            └────┬───────────────────────────┬─────────┘
                 │ Pick()                    │ forward
            ┌────▼─────┐               ┌─────▼────┐
            │ router   │◄──────────────│ backends │──► rpc nodes
            └──────────┘               └─────▲────┘
                                             │ alive + slot
            ┌────────────────────────────────┴─────────┐
            │ health checker (one goroutine per node)  │
            └──────────────────────────────────────────┘
```

config reload: SIGHUP -> validate -> swap snapshot atomically.

## layout

```
cmd/lb/          main binary
cmd/fakerpc/     fake solana rpc for tests and demo
internal/
  config/        yaml -> validated Config
  backend/       Backend, Pool snapshot
  router/        pick strategies
  jsonrpc/       parse envelope, classify method
  health/        active checker, slot lag ejection
  breaker/       passive circuit breaker
  proxy/         http handler, forward, retry, hedge, broadcast
  cache/         ttl cache, request coalescing
  ratelimit/     token bucket per client
  metrics/       prometheus collectors
docs/
deploy/          dockerfile, docker-compose
```

packages exist only when their milestone lands.

## milestones

### tier 1: correct balancer

- [x] M0 setup. go.mod, git, cmd/lb, 15-line reverse proxy to localhost:9001.
- [ ] M1 config. yaml, flag, validation, env expansion. internal/config.
- [ ] M2 pool and round-robin. internal/backend, internal/router. Picker interface in proxy.
- [ ] M3 operability. slog, request-id middleware, access log, server timeouts, graceful shutdown.
- [ ] M4 active health checks. internal/health. goroutine per backend, thresholds.
- [ ] M5 solana aware. internal/jsonrpc parse, cmd/fakerpc, getSlot health, slot-lag ejection.
- [ ] M6 metrics. internal/metrics, prometheus, admin port, /healthz /readyz /backends. docker-compose with grafana.

checkpoint: dockerfile, github actions, push v0.1.

### tier 2: smarter

- [ ] M7 picking. smooth weighted round-robin, p2c + ewma, rendezvous hash for sticky.
- [ ] M8 resilience. replace ReverseProxy with own forwarder, retry budget, circuit breaker.
- [ ] M9 method policies. broadcast sendTransaction, hedge reads.
- [ ] M10 hot reload. SIGHUP, atomic snapshot swap.

### tier 3: polish

- [ ] M11 cache + coalescing. ttl cache, singleflight.
- [ ] M12 rate limit. token bucket per client.
- [ ] M13 proof. chaos script, benchmarks, demo against devnet, v1.0.

## log

- 2026-11-13 M0 done. single root commit, repo live, python backend verified.
