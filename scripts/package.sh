#!/usr/bin/env bash
# Build the multi-arch container image locally (requires Docker Buildx).
set -euo pipefail
cd "$(dirname "$0")/.."

IMAGE="${IMAGE:-ghcr.io/bgriffin63/reelping}"
VERSION="${VERSION:-$(cat VERSION 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"

echo "==> building ${IMAGE}:${VERSION} for ${PLATFORMS}"
docker buildx build \
  --platform "${PLATFORMS}" \
  --build-arg "VERSION=${VERSION}" \
  --build-arg "COMMIT=${COMMIT}" \
  --build-arg "DATE=${DATE}" \
  -t "${IMAGE}:${VERSION}" \
  "${@:-}" .

echo "Done. To load a single-arch image locally, add: --load --platform linux/amd64"
