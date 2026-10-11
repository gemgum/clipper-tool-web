# Ide Pengembangan Lain (Masukan)

Fitur yang bisa dikembangkan setelah MVP jalan. Diurutkan dari yang paling
bernilai & mudah.

## Prioritas tinggi (nilai besar, effort sedang)
- **Cache transkrip** — jangan transkrip ulang video yang sama. Simpan hasil
  Whisper per file (hash). Hemat waktu & biaya besar.
- **Editor subtitle sebelum export** — user bisa koreksi teks/timing di GUI.
  Whisper kadang salah, ini fitur pembeda.
- **Preview player di GUI** — lihat klip + subtitle sebelum render final.
- **Auto judul + hashtag (LLM)** — sekalian minta Claude bikin judul catchy
  & hashtag per klip. Murah, sangat berguna untuk posting.
- **Progress & log realtime** — engine kirim progress ke GUI (SSE/websocket).

## Prioritas menengah
- **Batch / antrian** — proses banyak video sekaligus, satu per satu.
- **Template gaya subtitle** — preset (.ass): warna, font, animasi karaoke.
- **Preset export per platform** — TikTok/Reels/Shorts (resolusi, durasi max).
- **Manajer unduh model Whisper** — GUI bantu download model (tiny..large).
- **Profil konfigurasi** — simpan setelan (mode, model, gaya) yang sering dipakai.

## Prioritas lanjut (setelah matang)
- **Face-follow 9:16** — helper OpenCV C++ dipanggil Go (pakai skill C/C++).
- **Split-screen** untuk 2 orang (podcast).
- **Deteksi hook otomatis** — cari momen pembuka paling menarik.
- **Multi-bahasa** — Whisper sudah multibahasa; sesuaikan heuristik per bahasa.
- **Plugin scoring** — biar aturan viral bisa ditambah tanpa ubah inti.
- **B-roll / zoom otomatis** pada momen penting.

## Non-fitur tapi penting untuk proyek berkelanjutan
- **CLI dulu, GUI belakangan** — engine punya CLI supaya bisa dites tanpa GUI.
- **Config file** (mis. `clipper.yaml`) + env untuk API key (jangan hardcode).
- **README + CLAUDE.md** jelas untuk kontributor GitHub.
- **Test** untuk heuristik & parsing (bagian yang mudah rusak).
- **.gitignore** untuk model besar, output video, node_modules, API key.
- **Lisensi** (mis. MIT) kalau mau open-source.
- **Perhatikan lisensi ffmpeg** (GPL/LGPL) saat distribusi binary.
