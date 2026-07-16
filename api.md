# Image Roundup API

The API is served under `/api/v1`. It is read-mostly: scans are started with
`POST /api/v1/scan`, then clients poll scan status, summary counts, or filtered
image records.

Interactive docs are available from the running app:

- `/api/v1/docs`
- `/api/v1/openapi.json`

## Polling for Updates

The lowest-cost check is `GET /api/v1/summary`. Use `updatesAvailable` to decide
whether anything needs attention. This count includes digest drift for mutable
tags and newer compatible semver tags for immutable version tags.

```sh
curl -s http://localhost:8080/api/v1/summary
```

Example response:

```json
{
  "totalImages": 60,
  "upToDate": 59,
  "updatesAvailable": 1,
  "unknown": 0,
  "checkFailed": 0,
  "uniqueRegistries": 7,
  "lastScan": "2026-07-15T08:17:19Z"
}
```

With `jq`, this exits successfully when updates are present:

```sh
curl -fsS http://localhost:8080/api/v1/summary \
  | jq -e '.updatesAvailable > 0'
```

To fetch the actual update records:

```sh
curl -s 'http://localhost:8080/api/v1/images?status=update_available'
```

To fetch a concise update summary for automation:

```sh
curl -s http://localhost:8080/api/v1/summary/updates
```

Example response:

```json
{
  "count": 1,
  "lastScan": "2026-07-15T20:35:44Z",
  "updates": [
    {
      "image": "docker.io/fumbles/image-roundup:1.0.1",
      "currentVersion": "1.0.1",
      "latestVersion": "1.0.2",
      "namespace": "image-roundup",
      "workload": "Deployment/image-roundup",
      "containerName": "image-roundup",
      "management": {
        "tool": "Helm",
        "managedBy": "Helm",
        "helmReleaseName": "image-roundup",
        "helmReleaseNamespace": "image-roundup"
      },
      "updateReason": "newer_version_tag"
    }
  ]
}
```

Useful compact output:

```sh
curl -s http://localhost:8080/api/v1/summary/updates \
  | jq -r '.updates[] | "\(.namespace)\t\(.workload)\t\(.image)\t\(.currentVersion) -> \(.latestVersion // "digest changed")"'
```

## Triggering and Waiting for a Scan

Start a full scan:

```sh
curl -fsS -X POST http://localhost:8080/api/v1/scan \
  -H 'Content-Type: application/json' \
  -d '{}'
```

The scan runs asynchronously and returns `202 Accepted` immediately:

```json
{ "message": "scan started" }
```

Poll until `running` becomes `false`:

```sh
while true; do
  status="$(curl -fsS http://localhost:8080/api/v1/scan)"
  echo "$status" | jq .
  [ "$(echo "$status" | jq -r '.running')" = "false" ] && break
  sleep 3
done
```

Then check `/api/v1/summary` or `/api/v1/images?status=update_available`.

## Scoped Scans

Scan one namespace:

```sh
curl -fsS -X POST http://localhost:8080/api/v1/scan \
  -H 'Content-Type: application/json' \
  -d '{"namespace":"media"}'
```

Scan one workload:

```sh
curl -fsS -X POST http://localhost:8080/api/v1/scan \
  -H 'Content-Type: application/json' \
  -d '{"namespace":"media","workloadKind":"Deployment","workloadName":"plex"}'
```

If another scan is already active, the API returns `409 Conflict`:

```json
{ "error": "scan already in progress" }
```

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/docs` | Interactive API documentation |
| `GET` | `/api/v1/openapi.json` | OpenAPI 3.1 document |
| `GET` | `/api/v1/summary` | Summary counts and last scan time |
| `GET` | `/api/v1/summary/updates` | Concise update list for automation |
| `GET` | `/api/v1/images` | Image records; supports filters |
| `GET` | `/api/v1/images/{id}` | Single image record |
| `GET` | `/api/v1/registries` | Registry summary |
| `GET` | `/api/v1/scan` | Scan status |
| `POST` | `/api/v1/scan` | Start a full, namespace, or workload scan |
| `GET` | `/api/v1/settings` | Current runtime settings |
| `PUT` | `/api/v1/settings` | Replace runtime settings |
| `GET` | `/healthz` | Liveness probe |
| `GET` | `/readyz` | Readiness probe |
| `GET` | `/metrics` | Prometheus metrics |

## Image Filters

`GET /api/v1/images` supports these query parameters:

| Parameter | Example | Notes |
|-----------|---------|-------|
| `search` | `plex` | Searches image, namespace, workload, and container |
| `namespace` | `media` | Exact namespace match |
| `registry` | `docker.io` | Exact registry host match |
| `kind` | `Deployment` | Workload kind, case-insensitive |
| `status` | `update_available` | Exact status value |

Status values:

| Status | Meaning |
|--------|---------|
| `up_to_date` | Running digest matches the registry digest for the configured tag and no newer compatible version tag was found |
| `update_available` | Running digest differs from the registry digest for the configured tag, or a newer compatible version tag is available |
| `unknown` | Image could not be compared, usually because it lacks tag/digest data |
| `check_failed` | Registry lookup failed |

## Response Shapes

`GET /api/v1/scan`:

```json
{
  "running": false,
  "lastScan": "2026-07-15T08:17:19Z",
  "imageCount": 60,
  "errors": []
}
```

`GET /api/v1/images?status=update_available` returns an array of image records:

```json
[
  {
    "id": "lab:Deployment:dashy:dashy",
    "namespace": "lab",
    "workloadKind": "Deployment",
    "workloadName": "dashy",
    "containerName": "dashy",
    "configuredImage": "lissy93/dashy:latest",
    "registry": "docker.io",
    "repository": "lissy93/dashy",
    "tag": "latest",
    "runningDigest": "sha256:36b236d5a8c3...",
    "registryDigest": "sha256:8a839473ec3a...",
    "indexDigest": "sha256:ccfcb23451e2...",
    "latestTag": "4.4.5",
    "latestTagDigest": "sha256:8a839473ec3a...",
    "platform": "linux/amd64",
    "status": "update_available",
    "podNames": ["dashy-d45f565bb-fr64h"],
    "lastChecked": "2026-07-15T08:17:19Z"
  }
]
```

## Notes

- The API currently has no authentication layer of its own. Protect it with the
  deployment environment, Route/Ingress policy, or cluster auth if exposed.
- Settings updates change the running process. Environment variables remain the
  source of startup defaults.
- Scheduled scans run in the background based on `SCAN_INTERVAL_SECONDS`.
