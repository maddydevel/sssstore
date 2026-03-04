# Installation Guide

## Prerequisites

- Go 1.22+
- Linux/macOS/Windows shell access
- Optional: Docker and Kubernetes

## Option A: Run from source

```bash
git clone <repo-url>
cd sssstore
make build
./bin/sssstore init --config ./sssstore.json --data ./data
./bin/sssstore server --config ./sssstore.json
```

## Option B: Run directly with `go run`

```bash
go run ./cmd/sssstore init --config ./sssstore.json --data ./data
go run ./cmd/sssstore server --config ./sssstore.json
```

## Option C: Container image

```bash
make image
docker run --rm -p 9000:9000 -v $(pwd)/data:/var/lib/sssstore -v $(pwd):/etc/sssstore sssstore:local server --config /etc/sssstore/sssstore.json
```

## Option D: Kubernetes example

```bash
kubectl apply -f deploy/k8s/configmap.yaml
kubectl apply -f deploy/k8s/deployment.yaml
```

## Post-install checks

- Health: `curl http://localhost:9000/healthz`
- Readiness: `curl http://localhost:9000/readyz`
- Metrics: `curl http://localhost:9000/metrics`
