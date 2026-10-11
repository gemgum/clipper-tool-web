# 43 — Transkripsi lewat Google AI Studio

Diminta pemilik 9 Oktober 2026: selain whisper lokal, transkripsi bisa
dikerjakan Gemini lewat Google AI Studio. Dipilih per job, bukan cadangan.

## Cara memakai

- Kunci & model diisi DI APLIKASI: Settings → Exceptions → Video clips
  (mesin, transkripsi, model, API key dalam satu form). Bawaan model
  `gemini-3.5-flash`.
- CLI: `clipper run video.mp4 -transcriber aistudio`.
- API: `"transcriber": "aistudio"` di `options` job (`whisper` = bawaan).
- `GET /api/requirements` membawa `ai_studio: {key_set, model}`; kuncinya
  sendiri tidak pernah dikirim.

## Cara kerjanya

1. Audio diekstrak seperti biasa (WAV 16 kHz mono).
2. Dipotong per 10 menit, tiap potongan dikodekan Opus mono 24 kbps
   (≈ 1,8 MB per 10 menit) dan dikirim INLINE ke `generateContent`, berurutan.
   Jauh di bawah batas 20 MB per permintaan, dan balasan JSON satu potongan
   masih muat di jatah keluaran model.
3. Model diminta JSON berskema: `segments: [{start, end, text}]`, detik relatif
   potongan. Engine menjepitnya ke batas potongan, membuang teks kosong,
   mengurutkan, memotong tumpang-tindih, lalu menggeser ke waktu mutlak.
4. Hasilnya `types.Transcript` yang sama dengan whisper, jadi penjaga loop,
   koreksi, segmentasi, dan subtitle tidak berubah.

## Batas yang diketahui

- **Waktu per kata hanya PERKIRAAN.** Gemini tidak memberi timestamp per kata
  yang bisa dipercaya, jadi durasi tiap segmen dibagi ke katanya sebanding
  jumlah huruf. Mode karaoke/word dengan transkrip ini lebih kasar sinkronnya
  daripada whisper.
- Kalimat yang terbelah batas 10 menit bisa jadi dua segmen.
- Tanpa cadangan: kunci kosong, kunci ditolak, atau balasan tak terbaca
  menghentikan job dengan pesan sebabnya (notes/12). 429/5xx diulang lewat
  `httpx.Retry`.
- Kunci cache transkrip memakai model `aistudio:<model>`; cache whisper lama
  tetap terpakai (kuncinya tidak berubah).
- Kunci: satu, kunci mesin "Google AI Studio (Gemini)" (`GEMINI_API_KEY`).
  `AI_STUDIO_KEY` di `.env` DIBUANG 9 Oktober 2026 — pemilik: semua kunci AI
  diisi manual di aplikasi, bukan lewat env.
- Bawaan job klip dipilih di Settings → Exceptions → "Video clips ·
  Transcription" (`CLIPPER_TRANSCRIBER` + `AI_STUDIO_MODEL` di `.env`, 9 Oktober
  2026). Job yang mengirim `transcriber` sendiri tetap menang.

## Mesin OpenAI-compatible (9 Oktober 2026)

Selain whisper & AI Studio, transkripsi bisa memakai mesin OpenAI-compatible
dari Engines & Keys (OpenAI, Custom, mesin tambahan pengguna) lewat
`POST {base}/v1/audio/transcriptions` (`transcribe/openai.go`). Potongan audio
sama dengan AI Studio (10 menit, Opus), diminta `response_format=verbose_json`.

- Setelan: `CLIPPER_TRANSCRIBER=<id mesin>` + `CLIPPER_TRANSCRIBE_MODEL`
  (kosong = `whisper-1`). Model chat (glm-*, gpt-*) TIDAK bisa — yang
  dibutuhkan model ucapan-ke-teks.
- Model yang membalas teks tanpa segmen ditolak dengan pesan: satu subtitle
  sepanjang 10 menit lebih buruk daripada job yang berhenti.
- Waktu per kata = perkiraan (spreadWords), sama seperti AI Studio.
- Gemini tidak ditawarkan di sini: jalur OpenAI-nya tanpa endpoint audio; ia
  lewat pilihan AI Studio.
- Job menyimpan `transcribe_engine`; alamat & nama variabel kunci diisi ulang
  server saat job dibuat atau diulang (`job.Manager.Prepare`).
