#!/usr/bin/env bash
# Build engine Go + antarmuka statis.
#
# Keduanya dibangun sekaligus karena engine menyajikan antarmukanya sendiri dari
# gui/out (lihat engine/internal/api/webui.go). Engine tanpa gui/out tetap jalan,
# tapi yang muncul di browser cuma halaman "antarmukanya belum dibangun".
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "==> engine (Go)"
( cd "$ROOT/engine" && go build -o "$ROOT/bin/clipper" ./cmd/clipper )
echo "    bin/clipper"

if [ "${1:-}" = "-engine-only" ]; then
  echo "==> gui dilewati (-engine-only)"
elif command -v npm >/dev/null; then
  echo "==> gui (Next.js → statis)"
  ( cd "$ROOT/gui" && [ -d node_modules ] || npm install )
  ( cd "$ROOT/gui" && npm run build >/dev/null )
  echo "    gui/out"
else
  # Dilewati, bukan digagalkan: pengembang yang hanya menyentuh Go tidak perlu
  # memasang Node, dan CLI tetap berfungsi penuh tanpa antarmuka.
  echo "==> gui dilewati (npm tidak ada)"
fi

echo "Done."
