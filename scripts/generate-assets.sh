#!/usr/bin/env bash
# Regenerate all raster brand assets (PNG icons, favicon.ico, social card) from
# the canonical vector geometry. Deterministic: re-running produces identical
# output.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "==> generating raster assets into ./assets"
go run ./tools/genassets assets

# Keep the web UI's served copies of the SVGs in sync with the canonical ones.
cp -f assets/reelping.svg web/static/reelping.svg
cp -f assets/favicon.svg  web/static/favicon.svg

echo "Done. Assets:"
ls -1 assets
