#!/usr/bin/env bash
# Build engine Go + worker C++.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "==> engine (Go)"
( cd "$ROOT/engine" && go build -o "$ROOT/bin/clipper" ./cmd/clipper )
echo "    bin/clipper"

echo "==> worker (C++)"
cmake -S "$ROOT/worker" -B "$ROOT/worker/build" -DCMAKE_BUILD_TYPE=Release >/dev/null
cmake --build "$ROOT/worker/build" >/dev/null
echo "    bin/clipper-worker"

echo "Selesai."
