# Image Roundup

> **See what's running and what's changed.**

Image Roundup is a lightweight, read-only web application for Kubernetes and OpenShift that inventories container images running across a cluster and determines whether each workload is using the same image digest currently published by its configured registry tag.

## What it answers

- What container images are running?
- Which namespaces and workloads use each image?
- What tag is configured?
- What immutable digest is actually running?
- What digest does that registry tag resolve to now?
- Is an updated image available?

## Architecture

| Layer | Stack |
|---|---|
| Frontend | React, TypeScript, IBM Carbon Design System, Vite |
| Backend | Go, chi router, go-containerregistry, client-go |
| Deployment | Kubernetes / OpenShift, read-only RBAC |

```
┌─────────────────────────────────────────────────┐
│  Browser                                        │
│  React / Carbon UI                              │
└───────────────────┬─────────────────────────────┘
                    │ HTTP /api/v1/*
┌───────────────────▼─────────────────────────────┐
│  Go HTTP server  (:8080)                        │
│  ├─ /api/v1/summary                             │
│  ├─ /api/v1/images                              │
│  ├─ /api/v1/registries                          │
│  ├─ /api/v1/scan                                │
│  ├─ /api/v1/settings                            │
│  ├─ /healthz  /readyz  /metrics                 │
│  └─ /* → static/dist (React SPA)                │
├─────────────────────────────────────────────────┤
│  Scanner (background goroutine)                 │
│  ├─ K8s discovery (client-go)                   │
│  └─ Registry checks (go-containerregistry)      │
└─────────────────────────────────────────────────┘
```

## Getting started

### Prerequisites

- Go 1.22+
- Node 22+
- Access to a Kubernetes / OpenShift cluster

### Development

```bash
# Backend (out-of-cluster, uses ~/.kube/config)
IN_CLUSTER=false go run ./backend/cmd/server

# Frontend (dev server with proxy to :8080)
cd frontend && npm run dev
```

### Build

```bash
# Build backend
go build -o image-roundup ./backend/cmd/server

# Build frontend
cd frontend && npm run build
```

### Docker

```bash
docker build -t image-roundup:latest .
```

### Deploy to Kubernetes / OpenShift

```bash
# Create namespace, RBAC and ServiceAccount
kubectl apply -f deploy/k8s/rbac.yaml

# Deploy workload and Service
kubectl apply -f deploy/k8s/deployment.yaml

# OpenShift: create Route
oc apply -f deploy/k8s/route.yaml
```

## REST API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/summary` | Dashboard summary counts |
| GET | `/api/v1/images` | All image records (supports `?search=&namespace=&registry=&kind=&status=`) |
| GET | `/api/v1/images/{id}` | Single image record |
| GET | `/api/v1/registries` | Registry summary |
| GET | `/api/v1/scan` | Scan status |
| POST | `/api/v1/scan` | Trigger manual scan |
| GET | `/api/v1/settings` | Current settings |
| PUT | `/api/v1/settings` | Update settings |
| GET | `/healthz` | Liveness probe |
| GET | `/readyz` | Readiness probe |
| GET | `/metrics` | Prometheus metrics |

## Configuration (environment variables)

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8080` | HTTP listen address |
| `IN_CLUSTER` | `true` | Use in-cluster Kubernetes config |
| `KUBECONFIG` | `~/.kube/config` | Path to kubeconfig (out-of-cluster) |
| `SCAN_INTERVAL_SECONDS` | `28800` | How often to scan, in seconds |
| `REGISTRY_TIMEOUT_SECONDS` | `15` | Registry request timeout |
| `INCLUDED_NAMESPACES` | *(all)* | Comma-separated namespace allowlist |
| `EXCLUDED_NAMESPACES` | `kube-system,kube-public,kube-node-lease,openshift*` | Comma-separated namespace denylist; supports trailing `*` |
| `INCLUDE_COMPLETED_PODS` | `false` | Include succeeded/failed pods |
| `EXCLUDE_INTERNAL_REGISTRY` | `false` | Skip images that use the OpenShift internal registry service name |
| `THEME` | `system` | `system`, `light`, or `dark` |

## Status definitions

| Status | Meaning |
|--------|---------|
| `up_to_date` | Running digest matches the current registry tag digest |
| `update_available` | Running digest differs from the current registry tag digest |
| `unknown` | Cannot make a reliable comparison (no tag, digest-only, missing runtime status) |
| `check_failed` | Registry lookup failed (auth, connectivity, rate limiting) |

## RBAC

The application uses a `ClusterRole` with read-only verbs (`get`, `list`, `watch`) on:

- `pods`, `namespaces`
- `apps`: `deployments`, `statefulsets`, `daemonsets`, `replicasets`
- `batch`: `jobs`, `cronjobs`
- `apps.openshift.io`: `deploymentconfigs` (OpenShift only)

Secret access is **not** required unless registry pull-secret authentication is enabled in a future release.

## Out of scope (MVP)

- Automatic workload updates or pod restarts
- Multi-cluster management
- Vulnerability scanning
- Email / webhook notifications
- Semantic-version comparisons

## License

Apache 2.0
