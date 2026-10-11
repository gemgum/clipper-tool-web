#!/usr/bin/env bash
# Build engine Go + antarmuka statis.
#
# Keduanya dibangun sekaligus karena engine menyajikan antarmukanya sendiri dari
# gui/out (lihat engine/internal/api/webui.go). Engine tanpa gui/out tetap jalan,
# tapi yang muncul di browser cuma halaman "antarmukanya belum dibangun".
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"

# Nomor versi datang dari tauri.conf.json — SATU tempat.
#
# Ia yang tertulis di pemasang, dan workflow rilis sudah menolak tag yang tidak
# cocok dengannya. Tanpa langkah ini, biner hasil build lokal memakai bawaan
# `main.version` ("0.1.0-dev") dan panel Settings melaporkan angka yang tidak
# pernah dirilis siapa pun.
version() {
  sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
    "$ROOT/desktop/src-tauri/tauri.conf.json" | head -1
}

echo "==> engine (Go)"
VER="$(version)"
[ -n "$VER" ] || { echo "    versi tidak terbaca dari tauri.conf.json" >&2; exit 1; }
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
