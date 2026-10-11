#!/usr/bin/env bash
# Setup dependensi native: build whisper.cpp + unduh model.
# Jalankan sekali. Butuh: git, cmake, g++, curl/wget. Butuh internet.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
MODEL="${1:-small}"            # tiny|base|small|medium|large-v3
THIRD="$ROOT/third_party"
WHISPER_DIR="$THIRD/whisper.cpp"

mkdir -p "$ROOT/bin" "$ROOT/models" "$THIRD"

echo "==> 1/3 whisper.cpp"
if [ ! -d "$WHISPER_DIR" ]; then
  git clone --depth 1 https://github.com/ggerganov/whisper.cpp "$WHISPER_DIR"
fi
# Bersihkan cache whisper HANYA bila basi (folder dipindah/rename) — agar tak
# recompile whisper sia-sia setiap setup, tapi kebal pemindahan folder.
if [ -f "$WHISPER_DIR/build/CMakeCache.txt" ] && ! grep -q "$WHISPER_DIR/" "$WHISPER_DIR/build/CMakeCache.txt" 2>/dev/null; then
  echo "    (stale CMake cache after a folder move — cleaning)"
  rm -rf "$WHISPER_DIR/build"
fi
cmake -S "$WHISPER_DIR" -B "$WHISPER_DIR/build" -DCMAKE_BUILD_TYPE=Release >/dev/null
cmake --build "$WHISPER_DIR/build" -j --config Release --target whisper-cli
# Salin binary (lokasi bisa berbeda antar versi)
BIN="$(find "$WHISPER_DIR/build" -name whisper-cli -type f | head -1)"
cp "$BIN" "$ROOT/bin/whisper-cli"
# Salin shared library (.so) ke bin/ agar portabel (engine set LD_LIBRARY_PATH
# ke folder binary; tahan pemindahan folder & untuk distribusi).
cp -a "$WHISPER_DIR/build/bin/"*.so* "$ROOT/bin/" 2>/dev/null || true
echo "    whisper-cli + .so -> bin/"

echo "==> 2/3 model: $MODEL"
MODEL_PATH="$ROOT/models/ggml-$MODEL.bin"
if [ ! -f "$MODEL_PATH" ]; then
  URL="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-$MODEL.bin"
  if command -v curl >/dev/null; then
    curl -L --fail -o "$MODEL_PATH" "$URL"
  else
    wget -O "$MODEL_PATH" "$URL"
  fi
fi
echo "    model -> $MODEL_PATH"

echo "==> 3/3 subtitle fonts"
# Alamatnya ada di fetch-fonts.sh, dipakai bersama alur GitHub Actions.
"$ROOT/fetch-fonts.sh"

echo "Done. Next: ./build.sh && ./bin/clipper run <video>"
