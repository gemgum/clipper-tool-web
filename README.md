# Clipper

Alat kerja tim konten, versi web (dipakai lewat browser, engine di VPS):

| Tab | Fungsi |
|---|---|
| **Klip** | potong video panjang (1–4 jam) jadi klip 9:16 bersubtitle + skor "berpotensi viral"; preset gaya caption beranimasi |
| **Berita** | ubah artikel berita jadi kartu 1080×1920 (bisa carousel seluruh artikel) + caption siap posting |
| **Tulis** | gabungkan sampai 5 artikel sumber jadi satu artikel baru, dengan pagar fakta |
| **Caption** | caption pemancing dari ucapan video, satu `.txt` per video |
| **Tanda (Mark)** | bakar logo + judul ke video/foto (Shorts 9:16 atau feed 4:5 / 1:1) |
| **Riwayat** | semua hasil di satu daftar |

Konten sasaran: bahasa Indonesia. Antarmuka: Inggris / Indonesia.

---

## Pasang di komputer sendiri (pengembangan)

Linux, atau Windows lewat WSL (`wsl --install` di PowerShell Administrator, lalu
kerjakan semuanya di jendela Ubuntu).

```bash
sudo apt update
sudo apt install -y git cmake g++ ffmpeg curl golang-go nodejs npm   # Go ≥ 1.22, Node ≥ 18
gh repo clone gemgum/clipper-tool-web && cd clipper-tool-web         # repo privat
./setup.sh small     # whisper.cpp + model pengenal suara + font (sekali saja)
./build.sh           # engine Go (bin/clipper) + antarmuka statis (gui/out)
./bin/clipper serve  # buka alamat "open:" yang dicetaknya
```

Model `small` (±466 MB) untuk produksi bahasa Indonesia; `base` (±142 MB) untuk
uji cepat; `large-v3-turbo` paling akurat. `./setup.sh <model>` boleh diulang
untuk menambah model. Komponen yang kurang (ffmpeg, whisper, model, Chrome untuk
kartu berita) juga bisa dipasang dari halaman **Settings → Requirements**.

Mengembangkan antarmuka dengan hot reload: `cd gui && npm run dev` (port 3000),
engine tetap `./bin/clipper serve` di terminal lain.

## Mesin AI

Dipilih di **Settings** (kunci API diisi sekali di sana, tersimpan di `.env`):
Ollama lokal, Claude, ChatGPT, Gemini, DeepSeek, atau gateway OpenAI-compatible
milik sendiri. Halaman Klip juga punya mesin `heuristic` (tanpa AI). Engine
**tidak pernah diam-diam pindah mesin**: bila mesin yang dipilih gagal, job
berhenti dengan pesan penyebabnya. Contoh isi `.env` ada di `.env.example`.

## Versi web di VPS

`./bin/clipper serve -web -host <nama-domain>` di belakang Caddy/nginx, login
lewat sandi bersama atau SSO. Unit systemd dan konfigurasi proxy ada di
`deploy/`, susunan foldernya di `notes/42-versi-web.md`.

## CLI

```bash
./bin/clipper help        # semua perintah & flag
./bin/clipper run <video>             # klip dari satu video
./bin/clipper write <url>...          # artikel dari beberapa sumber
./bin/clipper caption <video>...      # caption per video
./bin/clipper watermark <video>...    # logo + judul
./bin/clipper fetch <youtube-url>     # unduh video YouTube
```

## Kalau ada masalah

| Pesan | Artinya |
|---|---|
| `whisper binary not found` / model tidak ditemukan | `./setup.sh <model>` belum jalan atau gagal; ulangi, atau pasang dari Requirements |
| Halaman "antarmukanya belum dibangun" | `./build.sh` dijalankan tanpa npm; pasang Node lalu ulangi |
| Kartu berita: browser tidak ditemukan | pasang Chrome/Chromium, atau set `CLIPPER_CHROME=/path/ke/chrome` |
| Kunci API ditolak | perbarui di Settings |
| `model "..." belum terpasang di Ollama` | `ollama pull <nama-model>` |

Transkrip dan koreksinya disimpan di `data/cache/`, jadi memproses ulang video
yang sama tidak mentranskrip dari nol.

## Struktur repo

```
engine/   Go (pustaka standar saja): HTTP API, pipeline, render, penyaji antarmuka
gui/      Next.js, dibangun jadi berkas statis gui/out yang disajikan engine
deploy/   systemd + Caddy/nginx untuk VPS
notes/    keputusan desain & alasannya (dirujuk dari kode)
scripts/  measure-ui.mjs — mengukur tata letak di 900×600 dan 1240×860
```

Panduan untuk agen/pengembang (aturan tampilan, komponen bersama, konvensi):
`CLAUDE.md`.
