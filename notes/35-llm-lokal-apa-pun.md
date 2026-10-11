# LLM lokal apa pun, bukan Ollama saja — 6 Agustus 2026

Pertanyaan pemilik proyek: bagaimana kalau pengguna memakai llama.cpp, LocalAI,
llamafile, Exo, vLLM, Aphrodite, atau LiteLLM? Dan bagaimana dengan model kecil
(1B–4B, bahkan 360M)?

## Yang menyatukan semuanya

Ketujuh server itu menyediakan API yang SAMA: `POST /v1/chat/completions`
bergaya OpenAI. Jadi yang dibutuhkan bukan tujuh integrasi melainkan satu bentuk
permintaan, plus satu cara mengenali mana yang sedang jalan.

`score/ollama/openai.go` — satu klien untuk semuanya:

- `POST {base}/v1/chat/completions`, `response_format: {type: "json_schema"}`
  saat bentuk balasan harus dijamin;
- `GET {base}/v1/models` untuk daftar modelnya;
- header `authorization: Bearer local` selalu dikirim — LiteLLM dan vLLM di
  belakang gateway mensyaratkannya walau kuncinya tidak diperiksa.

Ollama TETAP dipakai lewat API-nya sendiri (`/api/chat`), bukan lewat lapisan
/v1-nya: hanya di sana bentuk balasan bisa dipaksa dengan JSON Schema penuh, dan
justru itu yang membuat model lokal tetap membalas JSON rapi.

## Menemukannya sendiri

`kindOf()` menentukan jenis server: `/api/tags` berisi `models` → Ollama;
`/v1/models` berisi `data` → OpenAI-compatible. Kandidat portnya bertambah:
11434 (Ollama), 8080 (llama.cpp · llamafile · LocalAI), 8000 (vLLM · Aphrodite),
4000 (LiteLLM), 52415 (Exo), 1234 (LM Studio).

**Port terbuka saja tidak cukup.** `probeJSON` mensyaratkan 200 DENGAN JSON yang
memuat kunci yang diharapkan — kalau tidak, server pengembangan siapa pun di
port 8080 akan dikira LLM. Ada test-nya: server yang menjawab HTML diabaikan.

## Model kecil: ditandai, bukan ditolak

- Konteks < 8192 → dipakai, dengan catatan "potongannya dikirim lebih kecil".
  Yang ditolak hanya < 2048.
- Parameter < 6,5B → **dipakai**, dengan catatan: *"small model — fine for
  transcript correction, but it often returns empty fields when picking moments;
  the built-in heuristic is the safer choice there"*.

Dulu keduanya menolak modelnya. Itu masih benar secara pengukuran (notes/12),
tapi menolak berarti pengguna bermesin kecil tidak bisa memakai apa pun —
padahal koreksi transkrip kini menyesuaikan diri dengan memecah potongan, dan
mesin skor heuristik selalu ada sebagai gantinya.

Model CLOUD (`*-cloud`, 0 byte) tetap ditolak: ia bukan model lokal sama sekali.

## Yang BELUM dikerjakan, dan harus jujur disebut

- **Diuji dengan server tiruan, bukan dengan llama.cpp/vLLM sungguhan.** Bentuk
  permintaan & balasannya terbukti benar (`openai_test.go`), tapi perbedaan
  perilaku tiap server — dukungan `json_schema`, penamaan model, batas
  `max_tokens` — baru akan terlihat saat dicoba dengan yang asli.
- **Tidak ada UI untuk mengetik alamat server sendiri.** Yang ada: penemuan
  otomatis + `OLLAMA_HOST`. Kalau servermu di port tak lazim, setel env itu.
- **Server yang butuh kunci API sungguhan** (LiteLLM dengan auth menyala) belum
  didukung: kuncinya belum bisa disimpan dari GUI.

## Bagaimana engine tahu sebuah model "kecil"

**Ollama menyebutkannya sendiri.** `GET /api/tags` mengembalikan, per model:

```
llama3.1:latest   size=4.9GB  parameter_size=8.0B  context_length=131072  quantization=Q4_K_M  capabilities=[completion tools]
qwen2.5:latest    size=4.7GB  parameter_size=7.6B  context_length=32768   quantization=Q4_K_M  capabilities=[completion tools]
```

`parseParams` mengubah teksnya jadi angka: `"7.6B"` → 7,6 · `"270M"` → 0,27 ·
`"8x7B"` → 7 (yang aktif per token pada model MoE). Lalu `judge()` memutuskan,
berurutan:

| Diperiksa | Sumber | Akibat |
| --- | --- | --- |
| nama `*-cloud`, atau `size == 0` | daftar model | **ditolak** — bukan model lokal |
| `capabilities` tanpa `completion` | daftar model | **ditolak** — model embedding |
| `context_length < 2048` | daftar model | **ditolak** — tak cukup untuk satu segmen |
| `context_length < 8192` | daftar model | dipakai + catatan "potongan dikirim lebih kecil" |
| `parameter_size < 6.5B` | daftar model | dipakai + catatan "sering mengembalikan isian kosong saat memilih momen" |

Jadi "kecil" bukan tebakan: ia angka `parameter_size` yang dilaporkan Ollama,
dibandingkan dengan `minParams = 6.5` — batas dari pengukuran di `notes/12`
(model 4B membalas JSON valid tapi isian kosong; 7B mengerjakannya benar).

**Server bergaya OpenAI tidak menyebutkan apa pun.** `/v1/models` hanya memberi
`id`. Di situ satu-satunya petunjuk adalah NAMANYA, dan itu dipakai sebagai
petunjuk — `paramsFromName("Qwen2.5-1.5B-Instruct")` → 1,5B,
`"SmolLM2-360M"` → 360M — dengan dua pagar:

- ia hanya boleh menambah CATATAN, tidak pernah menolak model;
- angkanya harus berdiri sebagai potongan nama sendiri (`-1.5b-`), supaya "3.2"
  di `Llama-3.2` tidak terbaca sebagai 3,2 miliar. Ada test-nya, termasuk kasus
  yang harus mengembalikan 0.

Yang TIDAK bisa diketahui dari sana: panjang konteks. Untuk server itu
`num_ctx` tidak dikirim sama sekali (bukan parameternya), dan yang menjaga
adalah pemecahan potongan saat balasan gagal dibaca.
