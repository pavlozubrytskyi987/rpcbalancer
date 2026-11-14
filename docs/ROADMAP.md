# rpcbalancer roadmap

A Solana-aware JSON-RPC load balancer. Not "nginx but smaller" — a proxy
that understands the protocol it carries.

## 1. Why this is impressive (the pitch)

A generic round-robin proxy is a tutorial. Every RPC provider runs something
much more specific, because Solana RPC has problems a generic balancer
cannot see:

| Problem in production | What rpcbalancer does |
| --- | --- |
| A node answers HTTP 200 but is 80 slots behind — clients read stale state | Tracks each backend's slot, ejects nodes that lag the pool's tip |
| `sendTransaction` to one node may never land | Broadcasts to every healthy backend, returns first success |
| One slow node ruins p99 | Latency-aware picking (EWMA + power-of-two-choices) and hedged requests |
| A dying node takes errors for seconds before health checks notice | Passive circuit breaker on live traffic |
| Thousands of clients ask for the same blockhash | Short-TTL cache + request coalescing |
| Config change means a restart and dropped connections | Atomic hot reload on SIGHUP |

The four features that carry the interview:

1. **Slot-aware health.** Solana's own `getHealth` only reports unhealthy
   when a node is far behind (150 slots by default, about a minute). rpcbalancer
   polls `getSlot` on every backend and applies its own, tighter threshold
   relative to the highest slot in the pool.
2. **Method-aware routing.** The proxy parses the JSON-RPC body and applies
   a policy per method class: broadcast, hedged read, heavy, cacheable.
3. **Tail-latency engineering.** P2C with EWMA latency (the Finagle/Linkerd
   algorithm) plus hedged requests from Google's "The Tail at Scale".
4. **Proof.** A fake Solana node (`cmd/fakerpc`) with injectable latency,
   errors and slot lag, a chaos script, and measured before/after numbers
   in the README. Claims backed by a reproducible benchmark beat any
   feature list.

## 2. Architecture

```
                         ┌──────────────────────────────────────────┐
 client ── HTTP POST ──► │ middleware: request ID, log, metrics,    │
                         │             rate limit                   │
                         ├──────────────────────────────────────────┤
                         │ proxy.Handler                            │
                         │  1. read body (size-capped)              │
                         │  2. jsonrpc.Parse → method               │
                         │  3. policy for method class              │
                         │  4. cache lookup (cacheable only)        │
                         │  5. execute: pick → forward →            │
                         │     retry / hedge / broadcast            │
                         │  6. record latency + outcome             │
                         └───────┬───────────────┬──────────────────┘
                                 │ Pick()        │ forward
                         ┌───────▼──────┐   ┌────▼─────┐
                         │ router       │   │ backends │──► RPC nodes
                         │ (strategies) │◄──│ pool     │
                         └──────────────┘   └────▲─────┘
                                                 │ updates alive/slot
                         ┌───────────────────────┴──────────────────┐
                         │ health.Checker (one goroutine/backend)   │
                         └──────────────────────────────────────────┘
   SIGHUP ─► config.Load ─► validate ─► atomic swap of the live snapshot
```

### Package layout

```
cmd/lb/          main: parse flags, load config, wire dependencies, run
cmd/fakerpc/     fake Solana RPC node for tests, demos, chaos
internal/
  config/        YAML → validated Config. Leaf package, imports nothing internal
  backend/       Backend (URL, name, weight, live stats) and Pool snapshot
  router/        Picker strategies: round-robin, weighted, P2C-EWMA, rendezvous hash
  jsonrpc/       Parse request envelope, classify method. Pure, no I/O
  health/        Active checker: getHealth + getSlot, slot-lag ejection
  breaker/       Passive circuit breaker (closed → open → half-open)
  proxy/         HTTP handler, forwarding, retry, hedge, broadcast
  cache/         TTL cache + request coalescing (singleflight)
  ratelimit/     Token bucket per client
  metrics/       Prometheus collectors
docs/            ROADMAP.md, JOURNAL.md, benchmark results
deploy/          Dockerfile, docker-compose (lb + fakerpc×3 + Prometheus + Grafana)
```

Dependency rule: `cmd/lb` imports everything; `proxy` imports `backend` and
`jsonrpc`; nothing imports `proxy`; `config`, `jsonrpc`, `breaker`,
`ratelimit`, `cache` import nothing internal. Interfaces live with the
consumer — `proxy` declares the `Picker` it needs, `router` just has types
that happen to satisfy it.

Create packages only when their milestone arrives. No empty folders.

### Design decisions already made (agent: explain the why, don't relitigate)

- **Start with `httputil.ReverseProxy`, outgrow it in M8.** ReverseProxy
  cannot retry, hedge, or fan out. JSON-RPC bodies are small and fully
  buffered, so a hand-written forwarder on `http.Client` is simpler than
  bending ReverseProxy. "Why I replaced ReverseProxy" is a good story.
- **Immutable snapshot + atomic swap.** Config-derived state (pool,
  strategy, policies) is built into one immutable struct held in an
  `atomic.Pointer`. Requests load it once. Reload builds a new one and
  swaps. Per-backend live stats (latency, slot, in-flight) are atomics on
  the Backend itself.
- **Metric labels are allowlisted.** The `method` label comes from client
  input. Unknown methods are recorded as `other`, or an attacker can blow
  up Prometheus cardinality.
- **`sendTransaction` is safe to duplicate.** A signed transaction is
  identified by its signature; nodes deduplicate. That is why broadcast and
  retry are allowed for it, while the general rule is "only retry
  idempotent methods".
- **Batch requests (JSON array body)** go to a single backend under the
  default policy and are labelled `batch`. No splitting in v1.
- **Secrets from env.** Config supports `${VAR}` in backend URLs.

## 3. Milestones

Three tiers. Tier 1 alone is already a complete, resume-ready project.
Each milestone: design questions first, then code, tests, README, commit.

### Tier 1 — a correct load balancer

- [x] **M0 — Setup.** Repo at `~/Desktop/rpcbalancer`, `go mod init
  github.com/pavlozubrytskyi987/rpcbalancer`, `git init`, `.gitignore`,
  golangci-lint installed, Makefile (`run`, `test`, `lint`). Then the
  15-line hardcoded ReverseProxy in `cmd/lb/main.go`.
  *Go:* modules, package main, `net/http` server, `httputil.ReverseProxy`.
  *Ask:* What does a reverse proxy change in a request? What is `Host` vs
  `X-Forwarded-For`? Why `Rewrite` instead of `Director`?
  *Done when:* `curl localhost:8080` reaches `python3 -m http.server 9001`.

- [ ] **M1 — Config.** `internal/config`: `Load(path) (*Config, error)`,
  YAML, `-config` flag, validation (at least one backend, URLs parse,
  unique names), `${ENV}` expansion.
  *Go:* structs and tags, `os`, `flag`, error wrapping, sentinel errors,
  table-driven tests, `t.TempDir`.
  *Ask:* What belongs in validation vs. at use time? Pointer or value
  return? Which YAML library and is it still maintained? (Agent: check the
  current status of `gopkg.in/yaml.v3` vs `go.yaml.in/yaml` before
  recommending.)
  *Done when:* bad configs fail with a message that names the field.

- [ ] **M2 — Pool and round-robin.** `internal/backend`, `internal/router`.
  Proxy picks a backend per request. `Picker` interface declared in the
  consumer.
  *Go:* slices, methods, pointer vs value receivers, interfaces,
  `sync/atomic`, the race detector, `httptest.NewServer`.
  *Ask:* Why is `counter++` a data race? Mutex or atomic here? What happens
  with zero backends? Why does the interface live in `proxy`?
  *Done when:* 3 test backends receive an even split; `go test -race` passes.

- [ ] **M3 — Operability.** `slog` JSON logging, request-ID + access-log
  middleware, server timeouts, graceful shutdown on SIGINT/SIGTERM.
  *Go:* `http.Handler` and middleware as function composition, closures,
  `context`, `signal.NotifyContext`, `http.Server.Shutdown`, `defer`.
  *Ask:* Which server timeouts exist and what attack does each stop? What
  happens to in-flight requests on SIGTERM? How do you capture the status
  code in middleware?
  *Done when:* Ctrl-C during a slow request lets it finish, then exits 0.

- [ ] **M4 — Active health checks.** `internal/health`: one goroutine per
  backend, ticker, timeout per probe, mark down after N failures, up after
  M successes. Router skips dead backends.
  *Go:* goroutines, `time.Ticker`, `select`, context cancellation,
  `sync.WaitGroup.Go`, goroutine leaks, `testing/synctest` for time.
  *Ask:* Why thresholds instead of flipping on one failure? How does the
  checker stop on shutdown? All backends down — fail closed or try anyway?
  *Done when:* kill a backend → traffic leaves within the threshold;
  restart → it returns; no goroutine outlives shutdown.

- [ ] **M5 — Solana awareness.** `internal/jsonrpc` (parse envelope: single
  and batch, extract method, classify). `cmd/fakerpc` (answers `getHealth`,
  `getSlot`, `getLatestBlockhash`, `sendTransaction`; flags or an admin
  endpoint for latency, error rate, slot lag). Health checker switches to
  `getHealth` + `getSlot` and ejects backends more than N slots behind the
  pool's highest slot.
  *Go:* `encoding/json`, `json.RawMessage`, `io.LimitReader` /
  `http.MaxBytesReader`, re-reading a request body, native fuzzing.
  *Ask:* How do you parse only the fields you need? What must the proxy do
  with a malformed body? Compare slots at which commitment level, and why
  must all backends use the same one? What if the pool tip itself is wrong?
  *Done when:* fuzz test runs clean for a minute; a fakerpc set to lag 100
  slots is ejected while still returning HTTP 200.

- [ ] **M6 — Metrics.** `internal/metrics` + `/metrics` on a separate admin
  port, with `/healthz`, `/readyz`, `/backends` (JSON status). Request
  count and latency histogram by method/backend/outcome, backend up gauge,
  slot-lag gauge. docker-compose with Prometheus and Grafana, dashboard
  JSON committed.
  *Go:* a real third-party dependency, `go.mod`/`go.sum`, histograms vs
  summaries, injecting a registry instead of using the global default.
  *Ask:* Why a separate admin listener? Why allowlist the method label?
  Which buckets suit RPC latency?
  *Done when:* Grafana shows per-method p99 and slot lag during a demo run.

**Checkpoint: publish.** Dockerfile, GitHub Actions (lint, `test -race`,
build), README with the diagram. Push to GitHub. This is a shippable v0.1.

### Tier 2 — what separates it from a tutorial

- [ ] **M7 — Smarter picking.** Smooth weighted round-robin (the nginx
  algorithm), EWMA latency + in-flight count with power-of-two-choices,
  rendezvous hashing for sticky sessions by client IP. Strategy chosen in
  config. Benchmarks for each picker.
  *Go:* benchmarks with `b.Loop()`, `math/rand/v2`, hashing, lock
  contention, reading a CPU profile with `pprof`.
  *Ask:* Why P2C instead of "pick the fastest"? (herd behaviour) Why
  rendezvous hashing over `hash % n`? What does EWMA's decay constant trade?
  *Done when:* with one fakerpc at +200ms, P2C shifts most traffic away
  while round-robin does not — shown in Grafana.

- [ ] **M8 — Resilience.** Replace ReverseProxy with an own forwarder on a
  tuned `http.Transport`. Per-try timeout, overall deadline, retry on
  another backend for retryable methods with a retry budget,
  `internal/breaker` for passive ejection.
  *Go:* `http.Client`/`Transport` internals and connection pooling,
  context deadlines, state machines, `errors.As` on network errors.
  *Ask:* Which failures are retryable? Why does unlimited retry cause an
  outage? What is half-open for? Which Transport defaults are wrong for a
  proxy?
  *Done when:* fakerpc at 30% errors → client-visible error rate near zero,
  and retries stay under the budget.

- [ ] **M9 — Method policies.** Config maps method classes to behaviour.
  Broadcast `sendTransaction` to all healthy backends, return first
  success, cancel the rest. Hedge read methods: if no reply within the
  hedge delay, send a second request elsewhere and take the winner.
  *Go:* fan-out/fan-in, buffered channels, `select`, cancelling losers,
  not leaking goroutines or response bodies.
  *Ask:* Why is duplicating `sendTransaction` safe? What does hedging cost
  in extra load, and how do you cap it? Who closes the loser's body?
  *Done when:* with one slow fakerpc, p99 drops sharply with hedging on,
  at a measured extra-request rate under ~5%.

- [ ] **M10 — Hot reload.** SIGHUP → load → validate → build new snapshot →
  atomic swap. Invalid config is rejected and the old one keeps running.
  Health checkers for removed backends stop; new ones start; surviving
  backends keep their stats.
  *Go:* `atomic.Pointer[T]`, immutability as a concurrency strategy,
  lifecycle management of goroutines.
  *Ask:* Why swap a pointer instead of locking? What happens to a request
  mid-flight on the old snapshot? How do you prove nothing was dropped?
  *Done when:* a load test running through 20 reloads shows zero failures.

### Tier 3 — polish and proof

- [ ] **M11 — Cache and coalescing.** TTL cache for cacheable methods
  (immutable: `getGenesisHash`, finalized `getBlock`; short TTL:
  `getLatestBlockhash`, `getSlot`). Write singleflight by hand first, then
  compare with `golang.org/x/sync/singleflight`.
  *Go:* generics, `sync.Mutex` + map, cache keys from request params.
  *Ask:* What makes a response safe to cache? What is a cache stampede?

- [ ] **M12 — Rate limiting.** Token bucket per client IP (later per API
  key), JSON-RPC-shaped 429 errors, eviction of idle buckets.
  *Go:* time arithmetic, `synctest`, memory bounded maps.

- [ ] **M13 — Proof and ship.** Chaos script (kill, slow, lag backends
  during load), benchmark report with real numbers (round-robin vs P2C,
  hedging on/off, proxy overhead in µs), demo against devnet with real
  provider endpoints, README with diagram, results and a short GIF, tagged
  v1.0 release, write-up blog post from JOURNAL.md.

**Stretch (pick at most one or two):** WebSocket pubsub proxying
(`accountSubscribe`); read-your-writes consistency via `minContextSlot`
routing; API-key auth with per-key quotas; OpenTelemetry tracing; public
deployment on Fly.io.

## 4. What each milestone teaches

| Go skill | Where |
| --- | --- |
| Modules, layout, tooling | M0, M6 |
| Structs, methods, interfaces, errors | M1, M2 |
| `net/http` server and client in depth | M0, M3, M8 |
| Goroutines, channels, `select`, context | M4, M9 |
| Mutex vs atomic, race detector | M2, M7, M10 |
| Testing: table, httptest, fuzz, bench, synctest | M1, M2, M4, M5, M7 |
| JSON handling | M5 |
| Profiling and performance | M7, M13 |
| Generics | M11 |
| Shipping: Docker, CI, releases | checkpoint, M13 |

## 5. Progress log

Agent: one line per session — date, milestone, what was done, what is next.

- 2026-11-13 — M0 done. go.mod, cmd/lb/main.go, 15-line ReverseProxy to localhost:9001. curl → python backend verified.
