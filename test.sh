#!/usr/bin/env bash
set -euo pipefail

# ─── Helpers ─────────────────────────────────────────────────────────────────
red()   { echo -e "\033[0;31m$*\033[0m"; }
green() { echo -e "\033[0;32m$*\033[0m"; }
bold()  { echo -e "\033[1m$*\033[0m"; }
step()  { echo -e "\n\033[1;34m▶ $*\033[0m"; }

# ─── Pre-flight ───────────────────────────────────────────────────────────────
step "Pre-flight checks"

if ! command -v go &>/dev/null; then
  red "Go is not installed or not in PATH."
  exit 1
fi

if ! command -v npm &>/dev/null; then
  red "npm is not installed or not in PATH."
  exit 1
fi

if [ ! -f "go.mod" ] || [ ! -f "frontend/package.json" ]; then
  red "Run this script from the repo root."
  exit 1
fi

if [ ! -d "frontend/node_modules" ]; then
  red "frontend/node_modules not found. Run: cd frontend && npm ci"
  exit 1
fi

GO_CACHE_DIR="${GOCACHE:-${TMPDIR:-/tmp}/image-roundup-go-build-cache}"
mkdir -p "$GO_CACHE_DIR"

green "✓ Go $(go version | awk '{print $3}') found"
green "✓ npm $(npm --version) found"
green "✓ frontend dependencies found"
green "✓ Go build cache: $GO_CACHE_DIR"

# ─── Go checks ────────────────────────────────────────────────────────────────
step "Checking Go formatting"
unformatted="$(gofmt -l backend)"
if [ -n "$unformatted" ]; then
  red "Go files need formatting:"
  echo "$unformatted"
  exit 1
fi
green "✓ Go formatting is clean"

step "Running Go tests"
GOCACHE="$GO_CACHE_DIR" go test ./...
green "✓ Go tests passed"

# ─── Frontend checks ──────────────────────────────────────────────────────────
step "Running frontend lint"
(cd frontend && npm run lint)
green "✓ frontend lint passed"

step "Building frontend"
(cd frontend && npm run build)
green "✓ frontend build passed"

# ─── Done ─────────────────────────────────────────────────────────────────────
echo ""
bold "Test summary"
green "✓ All checks passed"
