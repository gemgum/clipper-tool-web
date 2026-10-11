# Arsitektur Clipper — Ringkasan

Proyek: memotong video panjang jadi klip pendek 9:16, otomatis subtitle, dan skor "berpotensi viral".

## Pipeline

```
Video panjang
   │
   1. Ekstrak audio ──────────► ffmpeg
   │
   2. Transkrip + timestamp ──► Whisper (word-level timing)
   │
   3. Segmentasi & scoring ───► heuristik (murah) → LLM (finalis)
   │
   4. Potong + reframe 9:16 ──► ffmpeg (+ face-follow opsional)
   │
   5. Burn subtitle ──────────► ffmpeg + .ass
   │
   └► Klip-klip pendek + skor + alasan
```

## Keputusan yang sudah diambil
- Bahasa: **Python**
- Scoring: **heuristik + Claude (LLM)**
- Output: **vertikal 9:16**
- Reframe: mulai dari **center crop** dulu, naik ke face-follow nanti
- Lokasi: masih diskusi (folder ini untuk catatan saja)

## Yang masih perlu diputuskan
- Jenis konten sumber (podcast / ceramah / berita?)
- Durasi video sumber biasanya berapa
- Ada GPU atau CPU saja
- Prioritas: MVP cepat vs fitur canggih
