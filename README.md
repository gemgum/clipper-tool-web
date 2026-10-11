# Clipper

Alat memotong video panjang (1–4 jam) jadi klip pendek 9:16 bersubtitle otomatis,
lengkap dengan skor "berpotensi viral". Fokus konten bahasa Indonesia.

Semua berjalan **di komputer sendiri**. Internet hanya dibutuhkan saat pemasangan
(dan bila memilih memakai Claude API).

---

# Panduan pemasangan dari nol (PC baru)

Ditulis untuk pemula — semua perintah tinggal disalin. Asumsi: Linux, atau
Windows lewat WSL.

## 0. Khusus Windows — pasang Linux dulu

Buka **PowerShell sebagai Administrator**, lalu:

```powershell
wsl --install
```

Restart PC, lalu buka aplikasi **Ubuntu** dari Start Menu. Semua langkah
berikutnya diketik di jendela Ubuntu itu — bukan di PowerShell.

## 1. Pasang alat yang dibutuhkan (±5 menit)

```bash
sudo apt update
sudo apt install -y git cmake g++ ffmpeg curl golang-go nodejs npm
```

Pastikan versinya cukup:

```bash
go version                  # harus 1.22 atau lebih baru
node -v                     # harus 18 atau lebih baru
ffmpeg -version | head -1
```

Kalau Go atau Node terlalu tua, itu satu-satunya yang perlu dipasang manual dari
situs resminya (golang.org dan nodejs.org).

## 2. Ambil proyeknya

Repo ini **privat**, jadi butuh login GitHub. Cara termudah — pasang GitHub CLI
lalu login sekali:

```bash
sudo apt install -y gh
gh auth login          # pilih GitHub.com → HTTPS → login lewat browser
gh repo clone gemgum/clipper-tool
cd clipper-tool
```

Alternatif tanpa akun GitHub: salin saja folder proyek dari PC lama (lewat
flashdisk atau jaringan). Folder `bin/`, `models/`, `third_party/`,
`gui/node_modules/`, dan `data/` **tidak perlu ikut** — semuanya dibuat ulang
oleh langkah 3 & 4.

## 3. Setup — cukup sekali seumur hidup (±10–20 menit)

```bash
./setup.sh base
```

Yang terjadi di belakang layar (biarkan sampai selesai):

1. mengunduh & mengompilasi **whisper.cpp** (mesin pengenal suara) — paling lama
2. mengunduh **model** pengenal suara
3. membangun **worker C++**
4. mengunduh **font** untuk subtitle

**Memilih model:**

| Model | Ukuran | Untuk apa |
|---|---|---|
| `base` | ±142 MB | coba-coba, cepat, akurasi seadanya |
| `small` | ±466 MB | **produksi bahasa Indonesia** — disarankan |
| `medium` | ±1,5 GB | paling akurat, jauh lebih lambat |

`./setup.sh small` bisa dijalankan lagi kapan saja untuk menambah model; yang
sudah ada tidak diunduh ulang.

## 4. Build aplikasinya (±1 menit)

```bash
./build.sh
```

Berhasil bila muncul `bin/clipper`, `bin/clipper-worker`, lalu `Selesai.`

## 5. Uji cepat lewat terminal

```bash
./bin/clipper run /path/ke/video.mp4 -model base -provider heuristic -max 3 -quality draft
```

> Di Windows/WSL, berkas `D:\video\test.mp4` diakses sebagai `/mnt/d/video/test.mp4`.

Hasilnya ada di folder `data/cli_<tanggal>/`. Kalau langkah ini jalan,
pemasangannya sehat.

## 6. Jalankan tampilan web (cara pakai sehari-hari)

Butuh **dua terminal**.

Terminal 1 — mesinnya:

```bash
cd ~/clipper-tool
./bin/clipper serve
```

Terminal 2 — tampilannya:

```bash
cd ~/clipper-tool/gui
npm install        # cukup sekali saja, ±2 menit
npm run dev
```

Buka browser ke **http://localhost:3000**.

Untuk pemakaian berikutnya cukup ulangi `./bin/clipper serve` dan `npm run dev` —
tanpa `setup.sh` maupun `npm install` lagi.

## 7. Opsional — mengaktifkan AI

Tanpa langkah ini pun aplikasi tetap jalan memakai mesin heuristik (berbasis
aturan, gratis, tanpa AI).

- **AI lokal (gratis)** — pasang [Ollama](https://ollama.com), lalu:
  ```bash
  ollama pull qwen2.5
  ```
  Di GUI: mode **offline** → mesin **AI lokal (Ollama)**.
  Model di bawah 7B umumnya terlalu kecil dan akan ditolak engine.

- **Claude (berbayar)** — di GUI pilih mode **hybrid**, tempel API key di panel
  Mesin AI. Bisa juga lewat berkas `.env` (salin dari `.env.example`).

---

# Kalau ada masalah

| Pesan | Artinya |
|---|---|
| `whisper binary tidak ditemukan` | `./setup.sh` belum dijalankan atau gagal di tengah — ulangi |
| `model whisper tidak ditemukan` | model yang dipilih belum diunduh → `./setup.sh small` |
| `ffmpeg: command not found` | langkah 1 terlewat |
| Halaman web kosong / "Engine tidak terjangkau" | terminal 1 (`clipper serve`) belum jalan |
| `API key Claude ditolak` | key salah/kedaluwarsa — perbarui di panel Mesin AI |
| `model "..." belum terpasang di Ollama` | jalankan `ollama pull <nama-model>` |
| `model terlalu kecil untuk prompt ini` | pakai model Ollama yang lebih besar (mis. qwen2.5) |

Engine **tidak pernah diam-diam mengganti mesin** yang Anda pilih. Bila mesinnya
gagal, job berhenti dan pesannya menyebut penyebabnya.

# Kebutuhan komputer

- **Disk**: ±2 GB (model `base`) sampai ±4 GB (model `medium`)
- **Prosesor**: transkripsi adalah tahap terberat dan memakai CPU — makin banyak
  core makin cepat. Gambaran kasar dengan model `base`: video 5 menit ≈ 1 menit
  proses; video 1 jam bisa 30 menit atau lebih.
- **Internet**: hanya saat langkah 1–3, dan bila memakai Claude.

Transkrip disimpan di cache (`data/cache/transcripts`), jadi memproses ulang
video yang sama tidak perlu transkripsi dari nol.

---

# Cara kerja singkat

```
ekstrak audio (ffmpeg) → analisis energi (worker C++) → transkripsi (whisper.cpp)
   → pemilihan momen (heuristik / Ollama / Claude) → render klip 9:16 + subtitle
```

Titik potong selalu berasal dari **batas kalimat pada transkrip**, bukan dari
analisis gambar.

## Mesin pemilih momen

| Mesin | Biaya | Cara kerja |
|---|---|---|
| heuristik | gratis | aturan: kata emosi, kata pemicu, kecepatan bicara, tanda titik, energi audio |
| Ollama (lokal) | gratis | LLM di komputer sendiri menentukan `start`/`end` tiap momen |
| Claude (API) | berbayar | sama seperti di atas, hasil paling baik |

Transkrip panjang dipecah bertumpang-tindih sebelum dikirim ke LLM (12 menit
untuk Ollama, 25 menit untuk Claude), dan momen yang terbelah di perbatasan
disambung otomatis.

## Arsitektur

```
Next.js (GUI web lokal)  --HTTP+SSE-->  Go (engine)  --stdin/NDJSON-->  C++ worker
                                             |
                                             +-- exec --> whisper.cpp, ffmpeg
```

- **engine/** — Go: orkestrasi, HTTP API, pemilihan momen, panggil whisper.cpp & ffmpeg
- **worker/** — C++: analisis audio (`features`); face-follow menyusul
- **gui/** — Next.js: antarmuka pengguna
- **notes/** — dokumentasi & keputusan desain (baca bila ingin mengembangkan)

---

# Rujukan perintah CLI

```bash
./bin/clipper run <video> [flag]
```

| Flag | Pilihan | Arti |
|---|---|---|
| `-mode` | offline \| hybrid | offline = lokal, hybrid = pakai Claude |
| `-model` | tiny\|base\|small\|medium\|large-v3 | model pengenal suara |
| `-provider` | claude \| ollama \| heuristic | pemilih momen (default ikut mode) |
| `-ollama-model` | mis. `qwen2.5` | model lokal untuk provider ollama |
| `-transcript-fix` | on \| off | LLM membenahi tanda baca, struktur & kata salah dengar sebelum klip dipotong (default on) |
| `-llm-model` | mis. `claude-haiku-4-5` | model Claude |
| `-duration` | auto\|30\|60\|90\|120\|180 | panjang klip (detik) |
| `-max` | angka | jumlah klip maksimum |
| `-min-score` | 0–100 | buang klip di bawah skor ini |
| `-sub-mode` | normal \| karaoke \| word | gaya subtitle |
| `-sub-speed` | slow \| normal \| dense | kecepatan tampil subtitle |
| `-save` | burn \| clean \| both | bersubtitle / polos / keduanya (+ `.srt`) |
| `-reframe` | center \| fit \| face_follow | Center of the Picture / Whole Picture (tanpa crop) / Follow Face |
| `-background` | blur \| black | isi ruang kosong saat `-zoom` di bawah 100 |
| `-zoom` | langkah 5, berhenti di 100 | **fit**: mulai 0 (seluruh video masuk), hanya bisa naik sampai 100 = isi penuh. **center**: mulai 100 (isi penuh), hanya bisa turun sampai 5. Default = titik awal modenya. |
| `-resolution` | 720p\|1080p\|1440p | resolusi keluaran |
| `-quality` | draft\|hd\|max | kecepatan vs ketajaman |
| `-fps` | 0\|24\|30\|60 | 0 = ikut sumber |
| `-out` | folder | lokasi simpan klip |

```bash
./bin/clipper serve [-addr 127.0.0.1:8787] [-jobs 1]
```

# Konfigurasi

Salin `.env.example` menjadi `.env`. Untuk mode hybrid isi `ANTHROPIC_API_KEY`
(atau cukup tempel lewat GUI — engine akan menuliskannya sendiri).
