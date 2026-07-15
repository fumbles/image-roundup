# Development Guide

This document is the working map for Image Roundup: how the app is structured,
how scans work, how to run it locally, and where to look when debugging.

## What Image Roundup Does

Image Roundup is a read-only Kubernetes/OpenShift app that inventories running
container images and compares the digest currently running in pods with the
digest currently served by the configured registry tag.

It answers:

- Which images are running in the cluster?
- Which namespace/workload/container uses each image?
- What tag is configured in the workload spec?
- What digest is running now?
- What digest does the registry currently serve for that tag?
- Is the running image up to date, changed, unknown, or failed to check?

The app does not update workloads. It is intentionally observational.

## Repository Layout

```text
backend/cmd/server        Go server entry point
backend/internal/api      HTTP API, static SPA serving, health probes
backend/internal/cache    In-memory store and optional NDJSON persistence
backend/internal/config   Environment variable loading
backend/internal/k8s      Kubernetes discovery, scan orchestration, OpenShift route support
backend/internal/models   Shared backend API/data models
backend/internal/registry OCI registry digest and tag resolution
backend/pkg/ociref        Image reference parser
frontend/src              React/TypeScript/Carbon UI
docs/screenshots          README screenshots
deploy/k8s                Kubernetes/OpenShift manifests
build.sh                  Host build + Docker buildx push
test.sh                   Formatting, Go tests, frontend lint/build
Dockerfile                Distroless runtime image
```

## Runtime Architecture

```text
Browser
  React + TypeScript + Carbon UI
      |
      | /api/v1/*
      v
Go HTTP server (:8080)
  chi router
  static frontend serving
  health/readiness/metrics
      |
      +--> cache.Store
      |      in-memory image records
      |      optional /data/records.ndjson persistence
      |
      +--> k8s.Scanner
             Kubernetes pod discovery
             registry digest checks
             latest compatible tag hints
```

The backend owns the source of truth. The frontend polls and filters data from
the REST API.

## Data Flow

1. `backend/cmd/server/main.go` loads env config and creates:
   - Kubernetes client
   - registry checker
   - cache store
   - API handler
   - background scanner loop
2. `k8s.Client.DiscoverImages` lists pods, resolves top-level workload owners,
   and builds `models.ImageRecord` values.
3. `k8s.Scanner.checkRecords` resolves registry digests with bounded
   concurrency. Duplicate lookup image references in the same scan are grouped,
   so a shared image is checked once and the result is fanned out to each
   workload record.
4. Each record gets a status:
   - `up_to_date`: running digest matches registry platform digest or index digest
   - `update_available`: running digest differs from registry digest
   - `unknown`: no reliable running digest or comparison data
   - `check_failed`: registry/API/auth/TLS lookup failed
5. If a record has `update_available`, the registry checker may list tags and
   select a compatible newer tag for display.
6. The store is replaced after full scans, or selectively replaced after scoped
   namespace/workload scans.

## Backend Components

### API

`backend/internal/api/handler.go` provides:

- `GET /api/v1/summary`
- `GET /api/v1/images`
- `GET /api/v1/images/{id}`
- `GET /api/v1/registries`
- `GET /api/v1/scan`
- `POST /api/v1/scan`
- `GET /api/v1/settings`
- `PUT /api/v1/settings`
- `/healthz`, `/readyz`, `/metrics`
- static SPA fallback

`POST /api/v1/scan` accepts an optional JSON body:

```json
{}
```

```json
{ "namespace": "media" }
```

```json
{
  "namespace": "media",
  "workloadKind": "Deployment",
  "workloadName": "plex"
}
```

Workload scans require all three fields. A scan returns `202 Accepted`; the
actual work runs in a goroutine. If another scan is active, the API returns
`409 Conflict`.

### Kubernetes Discovery

`backend/internal/k8s/discover.go`:

- lists pods across included namespaces or all namespaces
- skips excluded namespaces, including trailing `*` wildcard patterns
- skips completed pods unless enabled
- resolves ReplicaSet-owned pods back to their Deployment
- handles init containers separately
- can exclude OpenShift internal registry images
- supports scoped namespace/workload discovery

Default excluded namespaces are:

```text
kube-system
kube-public
kube-node-lease
openshift*
```

### Scanner

`backend/internal/k8s/scanner.go`:

- `Run`: full scan, replaces all records
- `RunScoped`: scoped scan, replaces only matching cached records
- `RunLoopWithStartupOptions`: first scan can use special options, then
  scheduled scans use normal options
- startup scan excludes the running Image Roundup workload itself when running
  in-cluster, which avoids detecting the previous rollout image while the new
  pod is starting
- registry checks run with bounded parallelism, currently capped at 8 workers
- duplicate lookup image references are de-duplicated per scan

OpenShift integrated registry handling lives here too. If an image uses the
internal registry service name, the scanner can detect the registry Route and
use the service account token for registry auth.

### Registry Checker

`backend/internal/registry/checker.go` uses `go-containerregistry` to:

- resolve tag digests
- resolve multi-arch indexes to linux/amd64 platform digests
- preserve index digest for display/comparison
- list semver-like tags for newer tag hints
- filter incompatible arch/variant/pre-release tags
- keep Postgres/PostgreSQL suggestions within the current major version
- keep LinuxServer images in their configured stream (`latest`, `develop`, or
  `nightly`)
- read Docker Hub auth from Docker config aliases such as `docker.io` when
  `go-containerregistry` asks for `index.docker.io`

Important behavior:

- Digest comparison uses platform digest and index digest.
- Latest tag hints are display-only; they do not affect status.
- Postgres major jumps are intentionally not suggested as normal updates.
- LinuxServer stream jumps are intentionally not suggested as normal updates.

### Cache and Persistence

`backend/internal/cache/store.go` is a mutex-protected in-memory store.

When `DATA_DIR` is set, records are loaded from and saved to:

```text
$DATA_DIR/records.ndjson
```

Persistence is record-only. Settings are currently process memory plus env
defaults; settings changes affect the running process but are not persisted
across pod restarts unless backed by env/config changes.

## Frontend Components

The frontend is React + TypeScript + Carbon.

Key files:

- `frontend/src/App.tsx`: app shell, Carbon header, routing, theme toggle
- `frontend/src/api.ts`: typed API client
- `frontend/src/types.ts`: frontend API shapes
- `frontend/src/pages/OverviewPage.tsx`: counts and attention list
- `frontend/src/pages/ImagesPage.tsx`: filters, search, image table
- `frontend/src/components/ImageDetail.tsx`: expanded record details and scoped refresh
- `frontend/src/pages/RegistriesPage.tsx`: registry summary
- `frontend/src/pages/SettingsPage.tsx`: scan/display settings
- `frontend/src/index.scss`: Carbon import and app-level styling
- `frontend/public/favicon.svg`: tab icon

Theme is stored through the settings API. The header toggle switches between
light and dark and dispatches a local `SETTINGS_SAVED_EVENT` so open pages stay
in sync.

Search in the Images page is client-side over the currently fetched records.
Namespace/registry/kind/status filters are URL-backed. Most status filters are
sent to the API; `status=unknown_failed` is a UI-only convenience filter that
combines `unknown` and `check_failed` locally.

Overview summary tiles deep-link into the filtered Images or Registries pages.
The expanded image details include scoped refresh actions and registry
inspection links for supported registries. The Images table uses fixed layout
and wraps long image/workload text so the common desktop view does not require
horizontal scrolling.

## Configuration

Environment variables:

| Variable | Default | Notes |
|---|---:|---|
| `LISTEN_ADDR` | `:8080` | HTTP bind address |
| `IN_CLUSTER` | `true` | Use in-cluster Kubernetes config |
| `KUBECONFIG` | empty | Used when `IN_CLUSTER=false`; falls back to client-go default |
| `DATA_DIR` | empty | Enables NDJSON record persistence |
| `STATIC_DIR` | `./static` | SPA directory used by the backend |
| `DOCKER_CONFIG` | Docker default | Directory containing registry auth `config.json` |
| `SCAN_INTERVAL_SECONDS` | `28800` | 8 hours |
| `REGISTRY_TIMEOUT_SECONDS` | `15` | Per registry request timeout |
| `INCLUDED_NAMESPACES` | empty | Comma-separated allowlist; empty means all |
| `EXCLUDED_NAMESPACES` | defaults above | Comma-separated denylist; supports trailing `*` |
| `INCLUDE_COMPLETED_PODS` | `false` | Include succeeded/failed pods |
| `EXCLUDE_INTERNAL_REGISTRY` | `false` | Skip OpenShift internal registry images |
| `THEME` | `system` | `system`, `light`, `dark` |

The Kubernetes deployment currently sets `DATA_DIR=/data`,
`DOCKER_CONFIG=/var/run/registry-auth`, and
`EXCLUDE_INTERNAL_REGISTRY=true`.

## Local Development

Install frontend dependencies once:

```bash
cd frontend
npm ci
```

Run backend out of cluster:

```bash
IN_CLUSTER=false go run ./backend/cmd/server
```

Run frontend dev server:

```bash
cd frontend
npm run dev
```

For the dev server to reach the backend, Vite should proxy `/api` to `:8080`
via `frontend/vite.config.ts`.

## Testing

Use the repo script:

```bash
./test.sh
```

It runs:

- Go format check
- `go test ./...`
- frontend lint
- frontend production build

The script sets a writable Go build cache under `$TMPDIR` by default, which is
useful in sandboxed environments.

For focused backend work:

```bash
go test ./backend/internal/k8s ./backend/internal/registry
```

For focused frontend work:

```bash
cd frontend
npm run lint
npm run build
```

## Build and Release

`build.sh` is the normal release path.

```bash
./build.sh [version-tag] [platform]
```

Defaults:

- image: `fumbles/image-roundup`
- version tag: `1.0.0`
- moving tag: `latest`
- platform: `linux/amd64`

Examples:

```bash
./build.sh 1.0.0
./build.sh 1.0.1
./build.sh 1.0.1 linux/amd64
```

The script:

1. runs `npm ci` and builds `frontend/dist`
2. cross-compiles a static linux/amd64 Go binary to `./image-roundup`
3. asks for confirmation
4. runs `docker buildx build --push` with both `:version-tag` and `:latest`

Pass a new immutable version tag for each release. `latest` is intentionally
reserved as the moving deployment tag and cannot be passed as the version tag.
The script does not auto-increment versions; pass `1.0.1`, `1.0.2`, etc.
explicitly for later patch releases.

The Dockerfile expects the binary and `frontend/dist` to already exist. It
copies both into a distroless static image.

## Kubernetes/OpenShift Deployment

Apply order:

```bash
kubectl apply -f deploy/k8s/rbac.yaml
kubectl apply -f deploy/k8s/pvc.yaml
kubectl apply -f deploy/k8s/deployment.yaml
oc apply -f deploy/k8s/route.yaml
```

The deployment:

- runs as a read-only scanner using `image-roundup` ServiceAccount
- mounts `/data` from PVC for cached record persistence
- optionally mounts registry auth at `/var/run/registry-auth`
- sets `DOCKER_CONFIG=/var/run/registry-auth`
- sets `EXCLUDE_INTERNAL_REGISTRY=true` by default
- exposes port `8080` through a Service

The RBAC is cluster-wide read-only for pods/namespaces and common workload
types. OpenShift-specific permissions support DeploymentConfigs, Routes, and
internal registry pull authorization.

## Registry Auth

The registry checker uses `authn.DefaultKeychain`, so Docker config auth works
when `DOCKER_CONFIG` points at a directory containing `config.json`.
There is also a Docker Hub alias fallback for configs that store credentials
under `docker.io` while registry requests are made to `index.docker.io`.

In Kubernetes, use `deploy/k8s/registry-auth.sh` as the starting point for
creating the registry auth secret. The deployment mounts that secret at:

```text
/var/run/registry-auth/config.json
```

OpenShift integrated registry lookups are special:

- images may be configured with the internal service hostname
- the app autodetects the external registry Route
- lookups use the Route host while preserving the configured image in the UI
- the service account token is used for registry auth

## Debugging

### Scan Is Slow

Scan time is mostly registry I/O:

- one digest lookup per image
- one extra tag-list lookup for images already flagged as update-available
- registry rate limits and TLS/auth failures can dominate runtime

Current scans use bounded concurrency with up to 8 registry workers. If scans
are still slow, inspect logs for repeated `registry check failed` messages and
look at which registries are timing out.

Useful checks:

```bash
kubectl logs -n image-roundup deploy/image-roundup
kubectl get pods -A
kubectl get route -n openshift-image-registry
```

### Many `unknown` Images

Common causes:

- the pod has no running digest yet
- the image is digest-pinned or tagless in a way that cannot be compared
- registry lookup failed before digest data was available
- the pod is starting/restarting while the scan runs

Expand an image row in the UI and compare:

- configured image
- configured tag
- running digest
- registry digest
- registry index digest
- platform

### OpenShift Internal Registry Errors

TLS errors against `image-registry.openshift-image-registry.svc:5000` usually
mean the app tried the internal service endpoint directly. The current code
should autodetect the default Route and use that host instead.

Unauthorized errors usually mean the service account lacks pull authorization
for that internal repository. Check:

```bash
oc auth can-i get imagestreams/layers --as system:serviceaccount:image-roundup:image-roundup
oc get route -n openshift-image-registry
```

### Theme Does Not Change

Theme state flows through:

```text
Settings API -> App.tsx -> Carbon Theme wrapper
```

The header toggle and Settings page both update settings and dispatch
`SETTINGS_SAVED_EVENT`. If the theme does not change, check the browser network
tab for `PUT /api/v1/settings` and confirm the response includes the new
`theme` value.

### Settings Changed But Scheduled Scans Ignore Them

Manual scans use the handler's current `scanOpts`, which are updated by
`PUT /api/v1/settings`.

The background scanner loop is started from `main.go` with the options present
at startup. If settings need to affect the already-running scheduled loop, that
loop should be changed to read options dynamically instead of receiving a copy.

## Common Development Tasks

### Add a New API Field

1. Add it to `backend/internal/models/image.go`.
2. Populate it in scanner/discovery code.
3. Add it to `frontend/src/types.ts`.
4. Render it in the relevant page/component.
5. Add focused tests if it affects status, filtering, or scan behavior.

### Add a New Setting

1. Add field to `models.Settings` and `DefaultSettings`.
2. Load env override in `config.Load` if needed.
3. Map it into `DiscoveryOptions` in `scanOptionsFromSettings`.
4. Add frontend type in `frontend/src/types.ts`.
5. Add control in `SettingsPage`.
6. Decide whether it must affect startup, manual, scheduled, or all scans.

### Adjust Latest Tag Selection

The logic is in `backend/internal/registry/checker.go`.

Add tests in `backend/internal/registry/checker_test.go`, especially for:

- architecture suffixes
- distro variants (`alpine`, `slim`, `bookworm`, etc.)
- prereleases
- major-version safety rules for stateful software
- stream/channel rules such as LinuxServer `latest`, `develop`, and `nightly`

### Adjust Scan Filtering

Discovery-level filtering lives in `backend/internal/k8s/discover.go`.

Cache replacement behavior for scoped scans lives in:

- `backend/internal/cache/store.go`
- `backend/internal/k8s/scanner.go`

## Gotchas

- Do not compare only manifest-list/index digest for multi-arch images; the
  running container usually reports the platform digest.
- Do not suggest Postgres major-version jumps as ordinary updates.
- Do not suggest LinuxServer stream jumps as ordinary updates.
- Digest-pinned images may not have a meaningful tag lookup.
- OpenShift internal registry service host and Route host are different; route
  lookup is needed for normal HTTPS registry access.
- Settings are not persisted to disk today.
- `build.sh` requires Docker/buildx and pushes after confirmation.
- `build.sh` does not auto-increment version tags.
- `Dockerfile` depends on host-built artifacts from `build.sh`.
