#!/usr/bin/env bash
# Build engine Go  
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "==> engine (Go)"
( cd "$ROOT/engine" && go build -o "$ROOT/bin/clipper" ./cmd/clipper )
echo "    bin/clipper"

echo "Done."
