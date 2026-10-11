# Mode Offline — Tools yang Diperlukan

Semua jalan di komputer sendiri, **tanpa biaya per-pemakaian**. Model diunduh
sekali di awal, setelah itu bisa full offline.

## Wajib

| Kebutuhan | Tool | Catatan |
|---|---|---|
| Potong/encode video, ekstrak audio, burn subtitle | **ffmpeg** | Inti dari semua. Wajib. |
| Runtime | **Python 3.10+** | |
| Transkrip + timestamp per kata | **faster-whisper** | Implementasi Whisper yang cepat (pakai CTranslate2). Alternatif: `whisper.cpp` (paling ringan, C++), atau `openai-whisper` (resmi tapi lebih lambat). |
| Backend ML untuk Whisper | **PyTorch** / CTranslate2 | Otomatis ikut saat install faster-whisper. |

## Untuk 9:16 face-follow (opsional, tahap lanjut)

| Kebutuhan | Tool |
|---|---|
| Deteksi wajah / tracking | **MediaPipe** (ringan) atau **OpenCV** + model, atau **YOLO** (ultralytics) |
| Manipulasi frame | **OpenCV** |

> Kalau cuma center-crop, ini TIDAK diperlukan — ffmpeg saja cukup.

## Untuk scoring viral tanpa API (offline penuh)

Ada 2 opsi:

1. **Heuristik saja** — tanpa model apa pun. Berbasis aturan:
   kata emosi, hook di awal, energi audio, durasi ideal. Gratis, ringan, cepat.

2. **LLM lokal** (kalau mau "pintar" tapi tetap offline):
   - **Ollama** + model seperti `llama3.1`, `qwen2.5`, atau `gemma2`.
   - Butuh RAM besar (8–16 GB+) dan idealnya GPU. Lebih lambat & kualitas
     di bawah Claude, tapi 0 biaya dan privat.

## Subtitle style
- Format **.srt** = subtitle biasa.
- Format **.ass** = bisa style viral (kata muncul satu-satu, highlight kuning).
  ffmpeg bisa burn keduanya. Untuk .ass tidak butuh tool tambahan.

## Ukuran model Whisper (offline)
`tiny` < `base` < `small` < `medium` < `large-v3`.
Makin besar makin akurat tapi makin lambat & butuh RAM/VRAM lebih.
Di CPU: `small` biasanya sweet spot. Di GPU: `large-v3` cepat & akurat.

## Kesimpulan mode offline paling ringan
**ffmpeg + Python + faster-whisper + heuristik.** Itu saja sudah bisa
menghasilkan klip 9:16 bersubtitle dengan skor. Tidak ada biaya sama sekali.
