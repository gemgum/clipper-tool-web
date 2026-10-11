# Keputusan Desain (Terkunci)

Profil developer: junior, stack C/C++, Go, Next.js. Target: CPU-only,
user-friendly, mudah maintain, mungkin diunggah ke GitHub.

## Stack

| Bagian | Pilihan | Alasan |
|---|---|---|
| Engine | **Go** + C/C++ via subprocess | Skill dikuasai; single binary; whisper.cpp/ffmpeg ramah-CPU |
| GUI | **Next.js**, web lokal (browser) | Skill dikuasai; tanpa tool baru; bisa upgrade ke Tauri nanti |
| Koneksi | Engine expose HTTP API di localhost | GUI panggil via API |
| Arsitektur | **Terpisah** (engine ↔ GUI) | Mudah maintain/test; engine dipakai ulang CLI & desktop (web BUKAN sasaran — CLAUDE.md) |

## Hybrid Go + C/C++ — level penggabungan

- **A. Subprocess (dipakai sekarang):** Go panggil binary whisper.cpp & ffmpeg.
  Go tetap murni → single binary, cross-compile mudah.
- **B. cgo binding:** hindari dulu — mematikan cross-compile & single binary.
- **C. Helper C/C++ sendiri (tahap lanjut):** untuk face-follow 9:16 pakai
  OpenCV (C++) dipanggil dari Go. Di sinilah skill C/C++ berguna.

Pembagian peran:
- Go = orkestrasi, HTTP API, job, heuristik scoring, panggil Claude API, config.
- C/C++ = beban berat & reusable (whisper.cpp, ffmpeg) + helper custom nanti.

## Mode engine (semua dibangun, sebagai config)

| Mode | Transkripsi | Scoring |
|---|---|---|
| Offline | whisper.cpp (lokal) | heuristik (atau LLM lokal) |
| Hybrid | whisper.cpp (lokal) | Claude API |
| Full online | STT API | Claude API |

CPU-only: default whisper.cpp model `small`/`base`, reframe center-crop dulu.

## Struktur folder (rencana)

```
clipper/
├── notes/            # dokumentasi & diskusi (ini)
├── engine/             # Go
│   ├── cmd/clipper/    # entrypoint: CLI + server
│   └── internal/
│       ├── transcribe/ # wrapper whisper.cpp
│       ├── clip/       # wrapper ffmpeg (cut, reframe 9:16, burn subtitle)
│       ├── score/
│       │   ├── heuristic/
│       │   └── llm/    # Claude client
│       ├── pipeline/   # orkestrasi end-to-end
│       ├── config/     # mode offline/hybrid/online
│       └── api/        # HTTP handlers
├── gui/                # Next.js
├── bin/                # binary eksternal (whisper.cpp, ffmpeg) — atau via PATH
├── models/             # model Whisper (.bin), gitignore
├── CLAUDE.md           # dibuat nanti (/init atau manual)
└── README.md
```

## Soal /init
Belum perlu sekarang (folder kosong). Urutan: kunci desain → scaffold →
baru /init atau tulis CLAUDE.md manual. notes/ + CLAUDE.md = pilar dokumentasi.
