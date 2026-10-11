#!/usr/bin/env bash
# Menyiapkan folder Clipper untuk Windows, dari Linux/WSL.
#
# Engine ini standard library saja — tanpa cgo — jadi Go bisa membuat .exe-nya
# langsung dari sini. Antarmukanya berkas statis, yang tidak peduli sistem apa
# pun. Jadi seluruh aplikasi (selain jendela Tauri) bisa diuji di Windows tanpa
# memasang apa-apa di sana.
#
# Yang TIDAK dihasilkan skrip ini: Clipper.exe berjendela + pemasang .msi. Itu
# butuh Rust + MSVC dan harus dibangun DI Windows — lihat notes/27.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
OUT="${1:-$ROOT/dist/windows}"

echo "==> engine (windows/amd64)"
# Versinya dari tauri.conf.json, sama seperti build.sh — biner uji di Windows
# harus menyebut angka yang sama dengan pemasangnya.
VER="$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
  "$ROOT/desktop/src-tauri/tauri.conf.json" | head -1)"
[ -n "$VER" ] || { echo "    versi tidak terbaca dari tauri.conf.json" >&2; exit 1; }
( cd "$ROOT/engine" && GOOS=windows GOARCH=amd64 \
  go build -ldflags "-X main.version=$VER" -o "$OUT/clipper.exe" ./cmd/clipper )

echo "==> gui (statis)"
if [ ! -f "$ROOT/gui/out/index.html" ]; then
  ( cd "$ROOT/gui" && npm run build >/dev/null )
fi

# Susunannya harus persis begini: engine mencari antarmuka & font DI SEBELAH
# binernya (<exe>/gui, <exe>/assets/fonts). Sama dengan susunan di dalam bundel
# Tauri, jadi yang diuji di sini benar-benar yang nanti dikirim.
rm -rf "$OUT/gui" "$OUT/assets"
cp -r "$ROOT/gui/out" "$OUT/gui"
mkdir -p "$OUT/assets"
cp -r "$ROOT/assets/fonts" "$OUT/assets/fonts"

echo
echo "Siap: $OUT"
echo "  clipper.exe   engine + antarmuka"
echo "  gui/          antarmuka statis"
echo "  assets/fonts/ font subtitle"
echo
echo "Salin foldernya ke Windows, lalu jalankan clipper.exe serve."
