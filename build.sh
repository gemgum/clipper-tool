#!/usr/bin/env bash
# Build engine Go + worker C++.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "==> engine (Go)"
( cd "$ROOT/engine" && go build -o "$ROOT/bin/clipper" ./cmd/clipper )
echo "    bin/clipper"

echo "==> worker (C++)"
# Clean build: cache CMake menyimpan path absolut, jadi hapus dulu agar kebal
# pemindahan/rename folder (build dir CMake tidak bisa dipindah).
rm -rf "$ROOT/worker/build"
cmake -S "$ROOT/worker" -B "$ROOT/worker/build" -DCMAKE_BUILD_TYPE=Release >/dev/null
cmake --build "$ROOT/worker/build" >/dev/null
echo "    bin/clipper-worker"

echo "Selesai."
