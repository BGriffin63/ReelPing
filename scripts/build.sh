#!/usr/bin/env bash
# Build the ReelPing binary with version metadata injected.
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${VERSION:-$(cat VERSION 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
OUT="${OUT:-dist/reelping}"

mkdir -p "$(dirname "$OUT")"
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w \
    -X github.com/BGriffin63/reelping/internal/version.Version=${VERSION} \
    -X github.com/BGriffin63/reelping/internal/version.Commit=${COMMIT} \
    -X github.com/BGriffin63/reelping/internal/version.Date=${DATE}" \
  -o "$OUT" ./cmd/reelping

echo "Built ${OUT} (version ${VERSION}, commit ${COMMIT})"
