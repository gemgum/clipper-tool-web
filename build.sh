#!/usr/bin/env bash
# Build engine Go + antarmuka statis.
#
# Keduanya dibangun sekaligus karena engine menyajikan antarmukanya sendiri dari
# gui/out (lihat engine/internal/api/webui.go). Engine tanpa gui/out tetap jalan,
# tapi yang muncul di browser cuma halaman "antarmukanya belum dibangun".
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"

# Nomor versi dari git (tag terdekat + commit), supaya panel Settings menyebut
# build yang benar-benar terpasang. Di luar checkout git: "dev".
echo "==> engine (Go)"
VER="$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)"
( cd "$ROOT/engine" && go build -ldflags "-X main.version=$VER" -o "$ROOT/bin/clipper" ./cmd/clipper )
echo "    bin/clipper $VER"

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
