# 12 — Kebijakan mesin skor: tanpa fallback, prompt detail, chunking, cache

Ronde 11 (28 Juli 2026). Perubahan sikap engine terhadap kegagalan mesin
pemilih momen, beserta alasannya.

## Prinsip: mesin pilihan pengguna dipakai apa adanya

Dulu engine diam-diam berpindah ke heuristik setiap kali LLM gagal. Akibatnya
pengguna mengira sedang memakai AI padahal tidak, dan penyebab kegagalan
tersembunyi di satu baris log yang mudah terlewat.

Sekarang: **tidak ada fallback**. Kalau mesin yang dipilih gagal, job berhenti
dan pesan errornya menyebut akar masalah. Heuristik hanya jalan bila memang
dipilih (`provider=heuristic`).

Contoh pesan yang dihasilkan (semua sudah diuji sungguhan):

| Situasi | Pesan |
|---|---|
| API key salah | `API key Claude ditolak — perbarui key di panel Mesin AI (invalid x-api-key)` |
| Model Ollama belum ada | `model "qwen2.5" belum terpasang di Ollama — klik "unduh model" di GUI atau jalankan ollama pull qwen2.5` |
| Ollama mati | `Ollama tidak terjangkau di http://localhost:11434 — pastikan sudah dipasang & jalankan ollama serve` |
| Balasan bukan JSON | `... membalas JSON yang tidak bisa dibaca: <error> — balasan: <300 karakter pertama>` |
| Model kekecilan | `... hanya membalas isian kosong (N momen tanpa judul & skor 0) — model terlalu kecil untuk prompt ini` |
| Batas waktu ngawur | `... membalas batas waktu yang tidak valid, semua ditolak: 900.0-1000.0 di luar durasi video (0.0-300.0)` |

Tanpa retry otomatis: satu kegagalan langsung berhenti (keputusan pengguna,
supaya masalah nyata tidak tertutupi percobaan ulang).

## Prompt: detail & sama untuk semua mesin

Dulu prompt Ollama sengaja disederhanakan karena model lokal lemah membalas JSON
rusak bila skemanya rumit (lihat ronde 9). Solusinya bukan menyederhanakan
prompt, melainkan **memaksa bentuk balasan lewat JSON Schema** di parameter
`format` Ollama (didukung sejak Ollama 0.5; di sini teruji pada 0.24).

Hasilnya prompt bisa sepenuhnya detail dan **identik** untuk Claude & Ollama
(`score/llm/prompt.go`) — jadi hasil kedua mesin bisa dibandingkan apel-ke-apel.
Isinya: aturan batas waktu (wajib dari timestamp yang ada, mulai/akhir kalimat,
tidak tumpang tindih), rentang durasi, definisi kelima dimensi skor, aturan
judul & tagar, plus aturan kesinambungan antar potongan.

### Temuan uji: prompt detail butuh model yang layak

Diuji dengan `qwen:latest` (Qwen 4B, 2,3 GB) pada transkrip 5 menit:

| Prompt | Hasil |
|---|---|
| Pendek (versi lama) | 1 momen, durasi 4 detik, skor 8 — bentuk benar, isi tidak berguna |
| Detail (versi sekarang) | JSON valid tapi **isian kosong**: `start 0, end 5, score 0, title ""` |

Kesimpulan: JSON Schema berhasil menghilangkan masalah *format*, tapi model 4B
tetap tidak sanggup mengerjakan *tugasnya*. Engine kini menolak balasan seperti
itu dan menyarankan model yang lebih kuat, alih-alih menghasilkan klip sampah.
**Saran: `ollama pull qwen2.5` (7B) untuk konten Indonesia.**

## Chunking dengan kesinambungan

Transkrip panjang dipecah sebelum dikirim ke LLM (`pipeline/chunk.go`):

- Panjang potongan: **12 menit** untuk Ollama (jendela `num_ctx` 8192),
  **25 menit** untuk Claude.
- **Tumpang tindih 2 menit**: ujung tiap potongan diulang di awal potongan
  berikutnya, sehingga momen di perbatasan tetap terlihat utuh minimal sekali.
- Timestamp yang dikirim **absolut** (detik asli video), model tidak perlu
  menghitung offset.
- Prompt menambahkan aturan: bila momen masih berlanjut melewati ujung potongan,
  set `"berlanjut": true` dan tulis `end` = ujung potongan.

Penggabungan (`mergeMoments`):

```
momen A [600-720] berlanjut=true   ┐
momen B [720-790]                  ┴──> satu klip [600-790]

momen dari area tumpang tindih yang sama ──> dibuang, skor tertinggi dipertahankan
tumpang tindih sebagian tanpa tanda berlanjut ──> awalnya digeser agar tak tabrakan
```

Ini menutup bahaya senyap sebelumnya: video >30 menit dulu terpotong diam-diam
oleh batas `num_ctx`, model tak pernah melihat bagian akhir, tapi job tetap
dilaporkan "sukses" dengan klip hanya dari paruh awal.

## Cache transkrip

`pipeline/cache.go`. Kunci = sidik jari isi video (ukuran + 4 MB awal + 4 MB
akhir) + model whisper + bahasa. Disimpan di `data/cache/transcripts/<kunci>.json`.

Penting karena kebijakan gagal-keras: transkripsi video 1 jam bisa puluhan
menit, dan tanpa cache setiap percobaan ulang mengulanginya dari nol.

Teruji: video uji 5 menit, jalan pertama **71 detik** → jalan kedua **0,4 detik**.

Bila cache kena, ekstraksi audio ikut dilewati — kecuali provider heuristik,
yang masih butuh WAV untuk fitur energi dari worker C++.

## Kode mati yang dihapus

`llm.Judge()` + `llm.Judgment` + prompt penilaian lamanya. Itu peninggalan
desain sebelum ronde 7 (heuristik memotong dulu, LLM hanya menilai potongan
jadi). Sejak ronde 7 pipeline hanya memakai `SelectMoments` — LLM yang
menentukan batas potong — tapi kode lamanya masih ada dan membingungkan.

## Deteksi model Ollama: dinamis, dinilai di engine

`/api/ollama/status` tidak lagi hanya mengirim daftar nama. Tiap model terpasang
dinilai kelayakannya di `score/ollama.judge()` dari metadata `/api/tags`:

| Syarat | Ditolak bila |
|---|---|
| Kemampuan | tidak punya `completion` (mis. model embedding) |
| Konteks | `context_length` < `numCtx` (8192) — prompt akan terpotong diam-diam |
| Ukuran | < 6,5 B parameter — sesuai temuan uji di atas (4B membalas isian kosong) |

Metadata kosong (Ollama lama) dianggap siap — jangan menuduh tanpa data.

GUI hanya menampilkan hasilnya: dropdown dibangun dari daftar terpasang
(`nama — 7.6B · Q4_K_M · 4.7 GB ✓ siap`), saran unduh hanya muncul untuk model
yang belum ada, dan status disegarkan tiap 15 detik + saat jendela kembali
fokus, jadi `ollama pull` dari terminal langsung terbaca.

**Bug yang diperbaiki**: perbandingan nama dulu memakai persamaan persis, jadi
`qwen2.5` (saran statis) tidak pernah cocok dengan `qwen2.5:latest` (nama asli
dari Ollama) dan model yang jelas-jelas terpasang tetap dicap "(perlu unduh)" —
padahal baris status di bawahnya memakai `startsWith` dan berkata "siap".
Sekarang semua perbandingan lewat satu helper `samaModel()` yang membuang tag.
