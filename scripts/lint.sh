#!/usr/bin/env bash
# Format check, vet, and (if installed) staticcheck. Also validates XML/SVG.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "==> gofmt"
fmtout=$(gofmt -l .)
if [ -n "$fmtout" ]; then
  echo "Not gofmt-formatted:"; echo "$fmtout"; exit 1
fi

echo "==> go vet"
go vet ./...

if command -v staticcheck >/dev/null 2>&1; then
  echo "==> staticcheck"
  staticcheck ./...
else
  echo "staticcheck not installed; skipping (go install honnef.co/go/tools/cmd/staticcheck@latest)"
fi

if command -v xmllint >/dev/null 2>&1; then
  echo "==> xmllint (templates + svg)"
  xmllint --noout templates/reelping.xml templates/ca_profile.xml
  for f in assets/*.svg web/static/*.svg; do xmllint --noout "$f"; done
else
  echo "xmllint not installed; skipping XML/SVG validation"
fi

echo "OK"
