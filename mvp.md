# Image Roundup — MVP Design Brief

## Product summary

**Image Roundup** is a lightweight web application for Kubernetes and OpenShift that inventories container images running across a cluster and determines whether each workload is using the same image digest currently published by its configured registry tag.

The application should answer:

* What container images are running?
* Which namespaces and workloads use each image?
* What tag is configured?
* What immutable digest is actually running?
* What digest does that registry tag resolve to now?
* Is an updated image available?
* Which images cannot be checked?

Image Roundup is initially **read-only**. It must not modify, restart, patch, or redeploy workloads.

---

## Target user

Kubernetes and OpenShift administrators who want a simpler, more visual alternative to Keel for container-image visibility and update detection.

The UI should be usable on desktop and mobile.

---

## Design direction

Use the **IBM Carbon Design System** and visually coordinate with the existing OCP Review site:

`https://ocp-review.yamlwrangler.com/#/home`

Desired visual characteristics:

* Clean Carbon application shell
* IBM Plex typography
* Dark and light theme support
* Compact but readable layout
* Blue accent color
* Status-focused dashboard
* Responsive mobile navigation
* Minimal visual clutter
* Technical information available without overwhelming the main view

Use Carbon components wherever practical rather than recreating equivalent controls. Carbon’s application shell, data table, tiles, tags, notifications, skeleton states, tooltips, overflow menus and status patterns should form the UI foundation.

---

## Primary navigation

### Overview

Dashboard containing summary tiles:

* Total images
* Up to date
* Updates available
* Unknown or check failed
* Unique registries
* Last scan time

Below the summary, show a compact list of images requiring attention.

### Images

Primary inventory page showing every discovered workload container image.

### Registries

Summary of registries in use, including connectivity and authentication status.

### Settings

Basic scan and display settings.

---

## Images page

Use a Carbon data table with:

| Column             | Description                                            |
| ------------------ | ------------------------------------------------------ |
| Status             | Up to date, update available, unknown, or error        |
| Image              | Repository name, such as `quay.io/example/app`         |
| Configured tag     | Tag from the workload specification                    |
| Running digest     | Digest associated with the running container           |
| Current tag digest | Digest to which the registry tag currently resolves    |
| Namespace          | Kubernetes namespace                                   |
| Workload           | Deployment, StatefulSet, DaemonSet, CronJob, Pod, etc. |
| Container          | Container name                                         |
| Registry           | Registry hostname                                      |
| Last checked       | Time of the latest comparison                          |

Provide:

* Free-text search
* Namespace filter
* Registry filter
* Workload-kind filter
* Status filter
* Sortable columns
* Refresh button
* Expandable rows
* Pagination or virtualized rendering for larger clusters

The default view should prioritize images with updates or errors.

---

## Status definitions

### Up to date

The running digest matches the digest currently returned by the configured tag.

### Update available

The running digest differs from the digest currently returned by the configured tag.

### Unknown

The application cannot make a reliable comparison, such as when:

* No tag is present
* The image uses only a digest
* Runtime status has not reported an image digest
* The image reference is malformed

### Check failed

The registry lookup failed because of authentication, connectivity, rate limiting, unsupported manifest data, or another registry error.

Never label an image as updated solely because a newer semantic version tag exists. The initial MVP compares the workload’s **configured tag** with its current registry digest.

Example:

```text
Configured image: ghcr.io/example/app:latest
Running digest:   sha256:111...
Registry digest:  sha256:222...
Result:           Update available
```

---

## Image details panel

Selecting or expanding an image should show:

* Full image reference
* Workload kind and name
* Namespace
* Pod names using the image
* Container name
* Configured tag
* Running image ID
* Running digest
* Current registry digest
* Manifest platform
* Registry response status
* Last successful check
* Last error, when applicable
* Copy buttons for image references and digests

The details view should explain the result in plain language.

Example:

> The workload is configured to use `latest`, but the running container digest differs from the digest currently assigned to that tag.

---

## Kubernetes discovery

Discover images from:

* Deployments
* StatefulSets
* DaemonSets
* ReplicaSets
* Jobs
* CronJobs
* Standalone Pods
* DeploymentConfigs on OpenShift, when available

Read both:

1. The configured image reference from the workload or pod specification.
2. The running image ID from pod container status.

Normalize duplicate results while preserving the relationship between images, workloads, namespaces, pods and containers.

Ignore completed pods by default, with an option to include them later.

---

## Registry comparison

Support common OCI-compatible registries, including:

* Docker Hub
* Quay
* GitHub Container Registry
* Red Hat registries
* Private OCI registries

Registry checks should:

1. Parse the configured image into registry, repository and tag.
2. Authenticate when credentials are available.
3. Request the image manifest for the configured tag.
4. Resolve the manifest digest.
5. Handle multi-architecture manifest indexes.
6. Select the digest matching the running pod’s platform when possible.
7. Compare the registry digest with the running image digest.
8. Store the result and timestamp.

Use Kubernetes image pull secrets when permitted, but do not expose secret values through the API, logs or UI.

Cache registry responses to avoid unnecessary requests and registry rate limits.

---

## Suggested architecture

### Frontend

* React
* TypeScript
* IBM Carbon React components
* Carbon icons
* Responsive single-page application
* React Router
* API client with loading, empty and error states

### Backend

* Go preferred
* Kubernetes client-go
* OpenShift API discovery where available
* OCI Distribution API client
* REST API serving inventory and scan status
* In-memory cache for the MVP
* Structured JSON logging
* Prometheus metrics endpoint

### Deployment

* Containerized application
* Kubernetes manifests or Helm chart
* ServiceAccount with read-only RBAC
* Deployment
* Service
* OpenShift Route when the Route API is available
* Optional Ingress for standard Kubernetes

---

## RBAC principle

Use the minimum permissions required.

The application should generally need read access to:

* Pods
* Deployments
* StatefulSets
* DaemonSets
* ReplicaSets
* Jobs
* CronJobs
* Namespaces
* DeploymentConfigs when available
* Referenced image pull secrets only when registry authentication is enabled

Secret access should be optional and clearly documented.

---

## MVP settings

Include settings for:

* Automatic scan interval
* Manual refresh
* Included namespaces
* Excluded namespaces
* Include completed workloads
* Registry request timeout
* Theme: system, light or dark
* Display short or full digests

Persist settings in a ConfigMap or application configuration file.

---

## UX requirements

* Show skeleton loading states during discovery.
* Show meaningful empty states.
* Do not display a false “up to date” result when data is incomplete.
* Display registry failures per image without failing the entire scan.
* Use text and icons in addition to color for status.
* Provide tooltips explaining tags and digests.
* Truncate digests visually but provide the full value through details and copy actions.
* Make filters usable on mobile.
* Meet Carbon accessibility guidance.
* Avoid excessive cards; use tables and progressive disclosure for dense technical data.

---

## Initial REST endpoints

```text
GET  /api/v1/summary
GET  /api/v1/images
GET  /api/v1/images/{id}
GET  /api/v1/registries
GET  /api/v1/scan
POST /api/v1/scan
GET  /api/v1/settings
PUT  /api/v1/settings
GET  /healthz
GET  /readyz
GET  /metrics
```

---

## Initial image record

```json
{
  "id": "namespace:deployment:app:container",
  "namespace": "media",
  "workloadKind": "Deployment",
  "workloadName": "example-app",
  "containerName": "app",
  "configuredImage": "ghcr.io/example/app:latest",
  "registry": "ghcr.io",
  "repository": "example/app",
  "tag": "latest",
  "runningDigest": "sha256:111",
  "registryDigest": "sha256:222",
  "platform": "linux/amd64",
  "status": "update_available",
  "lastChecked": "2026-07-14T18:00:00Z",
  "error": null
}
```

---

## MVP acceptance criteria

The first usable release is complete when it can:

1. Run inside an OpenShift or Kubernetes cluster.
2. Discover container images across authorized namespaces.
3. Show the configured tag and running digest.
4. Resolve the current digest for public registry tags.
5. Accurately classify images as up to date, update available, unknown or failed.
6. Search and filter the image inventory.
7. Show image and workload details.
8. Perform a manual rescan.
9. Render well on desktop and mobile.
10. Operate entirely read-only.

---

## Out of scope for the initial release

Do not implement these until the inventory and digest comparison are reliable:

* Automatic workload updates
* Workload patching
* Pod restarts
* Argo CD commits
* Git repository updates
* Semantic-version update recommendations
* Vulnerability scanning
* Multi-cluster management
* Email or webhook notifications
* User-defined approval workflows

Design the internal model so these capabilities can be added later without making them part of the MVP.

---

## Product identity

**Name:** Image Roundup

**Tagline:** See what’s running and what’s changed.

Suggested route:

```text
image-roundup.yamlwrangler.com
```

