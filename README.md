# rpcbalancer

Solana JSON-RPC load balancer in Go.

Sits in front of many Solana RPC nodes, picks a healthy one per request, forwards the call.

## Status

Day one. Config-driven reverse proxy to the first backend works.

## Run

```bash
# backends
mkdir -p /tmp/backend && cd /tmp/backend && python3 -m http.server 9001

# proxy
cp config.example.yaml config.yaml
go run ./cmd/lb -config config.yaml

# hit
curl localhost:8080
```

## Stack

Go 1.26, standard library, `gopkg.in/yaml.v3` for config. No frameworks.
