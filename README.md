# rpcbalancer

Solana JSON-RPC load balancer in Go.

Sits in front of many Solana RPC nodes, picks a healthy one per request, forwards the call.

## Status

Day one. Single-backend reverse proxy works. [Roadmap](docs/ROADMAP.md).

## Run

```bash
# backend
mkdir -p /tmp/backend && cd /tmp/backend && python3 -m http.server 9001

# proxy
go run ./cmd/lb

# hit
curl localhost:8080
```

## Stack

Go 1.26, standard library. No frameworks.

## License

MIT.
