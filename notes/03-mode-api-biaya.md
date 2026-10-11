# Mode API — "API yang Bagaimana?" & Perkiraan Biaya

## API yang dimaksud itu API yang bagaimana?

Ada **2 tempat** di pipeline yang BISA pakai API online. Keduanya opsional —
masing-masing punya versi offline.

### 1. API LLM (untuk menilai "berpotensi viral") — INI yang utama
Kita kirim **teks transkrip** tiap segmen ke sebuah model AI, lalu model
mengembalikan skor + alasan (hook, emosi, shareability, dll) dalam bentuk JSON.

- **Ini bukan API khusus "clipper"** — ini API model bahasa umum.
- Rekomendasi: **Claude API** (Anthropic). Model `claude-opus-4-8` (paling pintar),
  atau `claude-haiku-4-5` (murah & cepat) untuk hemat.
- Cara kerja: aplikasi kita kirim HTTP request berisi transkrip + instruksi
  penilaian → dapat balasan JSON. Butuh **API key** (bayar sesuai pemakaian token).
- Versi offline-nya: heuristik atau LLM lokal (Ollama).

### 2. API transkripsi (Speech-to-Text) — OPSIONAL
Kalau tidak mau jalankan Whisper sendiri, ada layanan STT online
(OpenAI Whisper API, Deepgram, AssemblyAI). **Ini justru yang paling mahal**
karena dihitung per menit audio.

- Versi offline-nya: faster-whisper (GRATIS, ini yang direkomendasikan).

> Ringkas: "pakai API" biasanya berarti **API LLM untuk scoring**. Transkripsi
> sebaiknya tetap offline pakai Whisper karena API STT mahal per menit.

---

## Perkiraan Biaya (Claude API untuk scoring)

Harga resmi Anthropic (per 1 juta token), per Juli 2026:

| Model | Input /1M | Output /1M |
|---|---|---|
| Claude Opus 4.8 | $5.00 | $25.00 |
| Claude Sonnet 5 | $3.00 ($2 promo s/d 31 Agu 2026) | $15.00 ($10 promo) |
| Claude Haiku 4.5 | $1.00 | $5.00 |

### Contoh: 1 video podcast durasi 1 jam
- Transkrip ~1 jam ngobrol ≈ 9.000 kata ≈ **~12.000 token**.
- Kita kirim transkrip + kandidat segmen sebagai input (~15.000 token),
  minta balasan skor+alasan untuk ~15 kandidat (~2.000 token output).

**Biaya per video 1 jam:**

| Model | Input | Output | **Total/video** |
|---|---|---|---|
| Opus 4.8 | $0.075 | $0.050 | **± $0.13** |
| Sonnet 5 (promo) | $0.030 | $0.020 | **± $0.05** |
| Haiku 4.5 | $0.015 | $0.010 | **± $0.025** |

> Artinya: **sangat murah.** Bahkan pakai model termahal (Opus), 1 video 1 jam
> hanya ~Rp 2.000-an. 100 video ≈ $13 (± Rp 200 ribu) pakai Opus, atau
> ~$2.5 pakai Haiku.

### Cara menekan biaya LLM lebih jauh
1. **Filter dulu dengan heuristik**, baru kirim finalis (mis. 15 segmen teratas)
   ke LLM — bukan seluruh transkrip mentah.
2. **Prompt caching** — kalau instruksi penilaian sama tiap kali, bagian itu
   di-cache (~0.1x harga). Hemat besar untuk volume banyak.
3. Pakai **Haiku** untuk penyaringan, **Opus** hanya untuk ranking akhir.

### Kalau MALAH pakai API transkripsi (tidak disarankan)
STT online ~$0.006/menit → **$0.36 per jam audio**. Ini ~3x lebih mahal dari
scoring Opus, dan berulang tiap video. Makanya transkripsi lebih baik offline.

---

## Ringkasan pilihan

| Bagian | Offline (gratis) | API (bayar) |
|---|---|---|
| Transkripsi | faster-whisper ✅ disarankan | STT API (mahal/menit) |
| Scoring viral | heuristik / Ollama | **Claude API** (murah, pintar) |

**Rekomendasi hemat + bagus:** transkripsi offline (Whisper) +
scoring pakai Claude API (Haiku untuk saring, Opus untuk ranking akhir).
Biaya efektif tinggal receh per video.
