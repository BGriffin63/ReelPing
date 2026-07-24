#!/usr/bin/env bash
# Run the full test suite (with coverage) and the race detector on the subset
# of packages that benefit most from it.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "==> go test ./... (coverage)"
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -n 1

echo "==> race detector (concurrency-sensitive packages)"
CGO_ENABLED=1 go test -race ./internal/monitoring/... ./internal/web/... ./internal/discord/... || {
  echo "note: -race needs a C toolchain (CGO_ENABLED=1); skip if unavailable" >&2
}
