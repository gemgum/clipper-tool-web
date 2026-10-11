# Kontrak API — Next.js ↔ Go ↔ C++

Keputusan: **Metode C** (worker C++ sendiri, aditif — whisper.cpp & ffmpeg tetap dipakai).
Bahasa sumber: **Indonesia** → default `language: "id"`, model Whisper `small`/`medium` (CPU).

> ## Pembaruan: kontrak di-Inggris-kan
>
> Catatan di bawah adalah rancangan awal. Sejak revamp penamaan, **seluruh
> kontrak memakai bahasa Inggris** — jalur, nama field JSON, nilai enum, dan
> nama event SSE. Yang berubah dari rancangan/implementasi lama:
>
> | Lama | Sekarang |
> | --- | --- |
> | `POST /api/news/analisis` | `POST /api/news/analyze` |
> | `?maks=` | `?max=` |
> | `?varian=polos` | `?variant=clean` |
> | `?latar=` (endpoint `/api/frame`) | `?background=` |
> | `options.latar: "blur"\|"hitam"` | `options.background: "blur"\|"black"` |
> | `subtitle.speed: "lambat"\|"padat"` | `subtitle.speed: "slow"\|"dense"` |
> | Article: `judul, ringkas, gambar, sumber, tanggal, terbit` | `title, summary, image, source, date, published` |
> | Feed: `nama, topik` | `name, topic` |
> | Card request: `artikel, gaya, rasio, rata, foto{geser_x,geser_y}, hashtag` | `article, style, ratio, align, photo{offset_x,offset_y}, hashtags` |
> | Gaya kartu `gelap\|terang\|kutipan` | `dark\|light\|quote` |
> | Perataan `kiri\|tengah\|kanan\|penuh` | `left\|center\|right\|justify` |
> | Analyze response: `artikel, paragraf, pilihan` | `article, paragraphs, selection` |
> | Selection: `kartu, peringkat{indeks,skor,alasan,teks,sumber}, mesin, catatan` | `card, rankings{index,score,reason,text,source}, engine, note` |
> | `/api/news/feeds`: `ada_browser, gaya, rasio, rata` | `has_browser, styles, ratios, aligns` |
> | Momen LLM: `"berlanjut"` | `"continues"` |
> | Card PNG: `kartu.png`, `sumber.txt` | `card.png`, `source.txt` |
> | Folder kartu: `data/kartu/kartu-<id>` | `data/cards/card-<id>` |
>
> **Ditegaskan:** `options.reframe` punya DUA nilai yang berdiri sendiri —
> `center` (Center of the Picture, potong tengah) dan `fit` (Whole Picture,
> seluruh resolusi video tanpa crop). `options.zoom` TERPISAH dari keduanya:
> 5..100 persen ukuran video di dalam bingkai, kelipatan 5. Latar hanya tak
> terlihat pada `center` + `zoom 100`.
>
> **Dibuang 5 Agustus 2026** (audit over-engineering): `face_follow`, mode
> `online`, `options.aspect`, `options.subtitle_style`, `options.score_engine`,
> `options.device`, dan `subtitle.karaoke`. Semuanya nilai yang tak pernah
> diset, alias tanpa isi, atau cabang yang tak terjangkau — jadi contoh JSON di
> bawah yang menyebutnya adalah rancangan awal, bukan kontrak yang berlaku.
> Keluaran selalu 9:16; warna subtitle diatur lewat `subtitle.color`; mesin skor
> ditentukan `mode` + `provider`.
>
> **`POST /api/jobs` menyemai default sebelum mendekode JSON.** Field yang tidak
> dikirim mempertahankan defaultnya, bukan jatuh ke nilai nol Go.
> Lihat `notes/15-sumbu-zoom.md`.
>
> **Baru:** `options.transcript_fix: "on"|"off"` pada `POST /api/jobs` (default
> `"on"`) — koreksi transkrip oleh LLM sebelum segmentasi, scoring, dan subtitle.
> Menambah tahap SSE `"correcting"` di antara `transcribing` dan `scoring`
> (pita progres 0.48–0.58). Lihat `notes/14-koreksi-transkrip.md`.
>
> **Baru:** parameter `lang` (query untuk GET, field body untuk POST) pada
> endpoint news & card. Menentukan bahasa teks yang DITULIS engine ke kartu —
> tanggal, kaki kartu, dan berkas pendamping. Default `en`; GUI mengirim bahasa
> antarmuka yang sedang dipilih. Isi artikel tidak pernah diterjemahkan.

## Alur data

```
Next.js (GUI) --HTTP+SSE--> Go (engine/API/orkestra) --stdin/NDJSON--> C++ worker (native)
                                  |
                                  +-- exec --> whisper.cpp, ffmpeg (binary C/C++ jadi)
```

- Go: manajemen job, panggil whisper.cpp & ffmpeg, heuristik, Claude API, setir worker C++.
- C++ worker: reframe 9:16, face-follow (OpenCV), analisis energi audio.
- ffmpeg: burn subtitle .ass + encode/mux final. Worker fokus visual, ffmpeg fokus codec.

---

## Kontrak 1 — Next.js ↔ Go (REST + SSE)

Base: `http://127.0.0.1:8787`

| Method | Endpoint | Fungsi |
|---|---|---|
| GET | /api/health | cek engine hidup |
| GET | /api/config | mode & default didukung |
| GET | /api/models | daftar model Whisper |
| POST | /api/models/download | unduh model `{ "name": "small" }` |
| POST | /api/jobs | buat job → `{ "id": "job_..." }` |
| GET | /api/jobs | daftar job |
| GET | /api/jobs/{id} | status + progress + hasil |
| GET | /api/jobs/{id}/events | SSE progress realtime |
| POST | /api/jobs/{id}/cancel | batalkan job |
| GET | /api/jobs/{id}/clips | daftar klip + skor |
| PATCH | /api/clips/{id}/subtitles | edit subtitle sebelum render |
| POST | /api/clips/{id}/render | render final klip |
| GET | /api/clips/{id}/file | unduh mp4 |
| GET | /api/clips/{id}/preview | streaming preview |

### POST /api/jobs (request)
```json
{
  "source": { "type": "path", "value": "/abs/video.mp4" },
  "mode": "hybrid",
  "language": "id",
  "transcription": { "model": "small", "device": "cpu" },
  "output": {
    "aspect": "9:16",
    "reframe": "center",
    "subtitle_style": "viral",
    "max_clips": 10,
    "target_duration": [20, 60]
  },
  "scoring": { "engine": "heuristic_llm", "llm_model": "claude-haiku-4-5", "min_score": 60 }
}
```
- `mode`: `offline` | `hybrid` | `online`
- `reframe`: `center` | `face_follow`
- `subtitle_style`: `plain` | `viral`
- `scoring.engine`: `heuristic` | `heuristic_llm` | `llm`

### GET /api/jobs/{id} (response)
```json
{
  "id": "job_a1b2",
  "status": "running",
  "stage": "scoring",
  "progress": 0.62,
  "error": null,
  "clips": []
}
```
- `status`: `queued` | `running` | `done` | `error` | `canceled`
- `stage`: `extracting` | `transcribing` | `segmenting` | `scoring` | `rendering`

### Clip
```json
{
  "id": "clip_09",
  "job_id": "job_a1b2",
  "start": 123.4, "end": 158.9, "duration": 35.5,
  "score": 82,
  "reasons": { "hook": 90, "emotion": 75, "clarity": 80, "shareability": 85, "standalone": 78 },
  "title": "Rahasia yang jarang diketahui...",
  "hashtags": ["#fyp", "#podcast"],
  "transcript": "...",
  "video_path": "clip_09.mp4",
  "video_path_raw": "clip_09_polos.mp4",
  "subtitle_srt": "clip_09.srt",
  "status": "rendered"
}
```
- `status`: `scored` | `rendering` | `rendered`
- Tidak ada `subtitle_path`: `.ass` cuma berkas antara di `tmp/` yang dihapus
  setelah dibakar ke video. Yang bertahan `.srt` (mode `clean`/`both`).
- `video_path_raw` & `subtitle_srt` hanya terisi bila mode simpan memintanya.

### PATCH /api/clips/{id}/subtitles (request)
```json
{ "segments": [ { "start": 0.0, "end": 1.8, "text": "Halo semuanya" } ] }
```

### SSE — GET /api/jobs/{id}/events
```
event: progress
data: {"stage":"transcribing","progress":0.30}

event: clip
data: {"id":"clip_09","score":82,"status":"scored"}

event: done
data: {"job_id":"job_a1b2","clips":8}

event: error
data: {"message":"ffmpeg gagal: ..."}
```

---

## Kontrak 2 — Go ↔ C++ worker (stdin JSON → stdout NDJSON)

Worker = subprocess. 1 request JSON di stdin, balas NDJSON di stdout, exit 0 = sukses.
Subprocess (bukan cgo) supaya Go tetap single-binary & cross-compile mudah.

### cmd: features (heuristik energi audio)
Request:
```json
{ "cmd": "features", "input": "/tmp/job_a1b2/audio.wav", "hop_ms": 100 }
```
Response (stdout):
```json
{"type":"result","rms":[0.12,0.34],"hop_ms":100}
```

### cmd: reframe (potong + 9:16)
Request:
```json
{
  "cmd": "reframe",
  "input": "/abs/video.mp4",
  "output": "/tmp/job_a1b2/clip_09_916.mp4",
  "start": 123.4, "end": 158.9,
  "target": { "w": 1080, "h": 1920 },
  "mode": "center",
  "options": { "smoothing": 0.3 }
}
```
Response (streaming NDJSON):
```json
{"type":"progress","value":0.5}
{"type":"result","output":"/tmp/job_a1b2/clip_09_916.mp4"}
```
- `mode: center` → crop tengah (ringan; bisa ffmpeg murni juga).
- `mode: face_follow` → OpenCV, lintasan crop ikut wajah. Bagian C++ utama.

### Error worker
```json
{"type":"error","message":"tidak bisa buka video"}
```

Pembagian render: worker hasilkan mp4 9:16 → ffmpeg (dipanggil Go) burn subtitle .ass + mux.
Worker fokus geometri/visual; codec diserahkan ke ffmpeg (jangan tulis ulang codec).

---

## Default bahasa Indonesia
- `transcription.language = "id"`
- Model: CPU → `small` (seimbang) atau `medium` (lebih akurat, lebih lambat).
  `large-v3` paling akurat tapi berat untuk CPU.
- Heuristik & prompt scoring perlu disesuaikan untuk bahasa Indonesia
  (kata emosi/hook dalam bahasa Indonesia).
