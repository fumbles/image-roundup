#!/usr/bin/env bash
set -euo pipefail

# ─── Config ──────────────────────────────────────────────────────────────────
IMAGE="${IMAGE:-fumbles/image-roundup}"
LATEST_TAG="${LATEST_TAG:-latest}"
DEFAULT_VERSION_TAG="${DEFAULT_VERSION_TAG:-1.0.0}"

# ─── Args ────────────────────────────────────────────────────────────────────
VERSION_TAG="${1:-$DEFAULT_VERSION_TAG}"
PLATFORM="${2:-linux/amd64}"

if [[ "${VERSION_TAG}" == "${LATEST_TAG}" ]]; then
  echo "Version tag cannot be '${LATEST_TAG}'. Pass an immutable version tag, e.g. ./build.sh 1.0.0"
  exit 1
fi

# ─── Helpers ─────────────────────────────────────────────────────────────────
red()   { echo -e "\033[0;31m$*\033[0m"; }
green() { echo -e "\033[0;32m$*\033[0m"; }
bold()  { echo -e "\033[1m$*\033[0m"; }
step()  { echo -e "\n\033[1;34m▶ $*\033[0m"; }

# ─── Pre-flight ───────────────────────────────────────────────────────────────
step "Pre-flight checks"

if ! command -v docker &>/dev/null; then
  red "Docker is not installed or not in PATH."
  exit 1
fi

if ! docker info &>/dev/null; then
  red "Docker daemon is not running. Start Docker Desktop and try again."
  exit 1
fi

if ! command -v go &>/dev/null; then
  red "Go is not installed or not in PATH."
  exit 1
fi

if [ ! -f "frontend/package.json" ]; then
  red "frontend/package.json not found. Run this script from the repo root."
  exit 1
fi

green "✓ Docker is running"
green "✓ Go $(go version | awk '{print $3}') found"
green "✓ frontend/package.json found"

# ─── Build React frontend ─────────────────────────────────────────────────────
# Build on the host so the Dockerfile never needs Node under QEMU.
# (QEMU + node:alpine is known to segfault on amd64 cross-builds from ARM hosts.)
step "Building React frontend"
(cd frontend && npm ci --prefer-offline && npm run build)
green "✓ frontend/dist is ready"

# ─── Build Go backend ─────────────────────────────────────────────────────────
# Cross-compile a static linux/amd64 binary on the host.
# The Dockerfile just COPYs it — no Go toolchain needed in the image.
step "Building Go backend (linux/amd64, static)"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags="-s -w" -o image-roundup ./backend/cmd/server
green "✓ image-roundup binary ready ($(du -sh image-roundup | cut -f1), linux/amd64)"

# ─── Summary ─────────────────────────────────────────────────────────────────
echo ""
bold "Build summary"
echo "  Image    : ${IMAGE}:${VERSION_TAG}"
echo "  Also tag : ${IMAGE}:${LATEST_TAG}"
echo "  Platform : ${PLATFORM}"
echo "  Context  : $(pwd)"
echo ""
read -r -p "Proceed with docker build & push? [y/N] " confirm
[[ "$confirm" =~ ^[Yy]$ ]] || { echo "Aborted."; exit 0; }

# ─── Docker build & push ──────────────────────────────────────────────────────
step "Building and pushing ${IMAGE}:${VERSION_TAG} and ${IMAGE}:${LATEST_TAG}"

if docker buildx version &>/dev/null; then
  docker buildx build \
    --platform "$PLATFORM" \
    -t "${IMAGE}:${VERSION_TAG}" \
    -t "${IMAGE}:${LATEST_TAG}" \
    --push \
    .
else
  red "buildx is required for multi-platform builds."
  red "Install it with: docker buildx install"
  exit 1
fi

# ─── Done ─────────────────────────────────────────────────────────────────────
echo ""
green "✓ Done!"
echo ""
echo "  Pull    : docker pull ${IMAGE}:${VERSION_TAG}"
echo "          : docker pull ${IMAGE}:${LATEST_TAG}"
echo "  Run     : docker run -p 8080:8080 ${IMAGE}:${VERSION_TAG}"
echo "  Open    : http://localhost:8080"
echo ""
echo "  Deploy  : kubectl apply -f deploy/k8s/"
echo "  Rollout : kubectl rollout restart deployment/image-roundup -n image-roundup"
echo ""
echo "  Hub     : https://hub.docker.com/r/fumbles/image-roundup"
