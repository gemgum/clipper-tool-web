# CLAUDE.md — Panduan untuk agen/sesi berikutnya

Proyek **Clipper**: memotong video panjang (1–4 jam) jadi klip pendek 9:16
bersubtitle otomatis + skor "berpotensi viral". Konten target: **bahasa Indonesia**.

## Arsitektur (3 lapis)

```
desktop/ (Tauri)  --membuka satu alamat-->  engine/ (Go)  --exec-->  whisper.cpp, ffmpeg
                                              ^     |
                            gui/ (Next.js) ---+     +-- menyajikan gui/out (statis)
                            HTTP + SSE
```

- **engine/** (Go) — otak: HTTP API, orkestrasi pipeline, scoring, pemasangan
  komponen, dan penyaji antarmuka. Standard library saja (tanpa dependency
  eksternal).
- **gui/** (Next.js) — enam tab: `/` klip video (form → progress SSE → daftar
  klip), `/news` kartu berita (tempel link / jelajah RSS → kartu PNG),
  `/writer` pembuat berita (sampai 5 artikel sumber → satu artikel baru +
  pagar fakta, lihat `notes/38`), `/captions` pembuat caption (satu video atau
  sekaligus banyak → satu `.txt` per video, lihat `notes/40`), `/watermark`
  gambar + headline dibakar ke video yang SUDAH 9:16 (lihat `notes/41`), dan
  `/requirements` status & pemasangan komponen. Ada pemilih bahasa antarmuka
  EN/ID di bilah navigasi (default EN, disimpan di localStorage). Dibangun jadi
  berkas statis (`gui/out`) yang disajikan engine.
- **desktop/** (Tauri) — jendela aplikasi. Setipis mungkin: menjalankan engine,
  membaca alamat yang dicetaknya, membuka alamat itu. Lihat `notes/27`.
- **worker/** (C++) — sudah dibuang (lihat `notes/20`); RMS energi audio kini di
  paket Go `audio`.
- **notes/** — diskusi & keputusan desain (baca 01–07 untuk konteks).

## Perintah penting

```bash
./setup.sh [base|small|medium|large-v3|large-v3-turbo]   # build whisper.cpp + unduh model
./build.sh                       # build engine (Go) + gui statis (gui/out)
./bin/clipper run <video> [-model -duration -max -min-score \
                           -reframe center|fit -background blur|black -zoom 5..200 \
                           -sub-mode normal|karaoke|word -sub-speed slow|normal|dense \
                           -transcript-fix on|off -terms "Londo Ireng,Mahfud MD" \
                           -save burn|clean|both \
                           -watermark-image logo.png \
                           -watermark-x -watermark-y \
                           -watermark-width -watermark-height \
                           -watermark-at -watermark-for \
                           -headline "..." -headline-llm \
                           -headline-x -headline-y -headline-size]   # CLI
./bin/clipper write <url>... [-provider claude|ollama -ollama-model -lang -out]
                                 # pembuat berita: sampai 5 artikel → satu artikel
./bin/clipper caption <video>... [-engine -model -minutes 5 -variants 3 \
                                  -terms -whisper -out]
                                 # caption: tiap video → <nama video>.txt
                                 # ditulis DI SEBELAH videonya; -out mengumpulkan
./bin/clipper watermark <video>... [-watermark-image logo.png \
                                -watermark-x -watermark-y \
                           -watermark-width -watermark-height \
                                -watermark-at -watermark-for \
                                -headline "..." -headline-x -headline-y \
                                -headline-size -font -quality -out]
                                 # tiap video 9:16 → <nama>_watermarked.mp4
                                 # yang bukan 9:16 DITOLAK per berkas, tidak dipotong
./bin/clipper serve              # API + GUI di 127.0.0.1:8787 (port acak bila terpasang)
                                 # banner mencetak satu alamat "open:" — tinggal dibuka
./bin/clipper serve -token on    # + kunci sesi (otomatis menyala saat terpasang)
cd gui && npm run dev            # GUI mode pengembangan di 3000 (hot reload)
cd desktop && npm run dev        # jendela aplikasi (Tauri) — butuh Rust
```

Tab kartu berita butuh Chrome/Chromium (Edge bawaan Windows juga bisa). Engine
mencarinya sendiri; timpa dengan `CLIPPER_CHROME=/path/ke/chrome`.

Di tab itu LLM **boleh menulis** ringkasan kartu dan caption (keputusan pemilik
9 Oktober 2026, `DESIGN-NEWSCARD.md`), TAPI setiap tulisannya diperiksa pagar
fakta `writer.CheckText`: angka, kutipan, dan nama yang tidak ada di artikel
ditandai di bawah isiannya. Sebelumnya LLM hanya memilih nomor paragraf —
riwayatnya di `notes/13-kartu-berita.md`.

## Alur pipeline (engine/internal/pipeline)

extract audio (ffmpeg) → features (RMS di engine) → transkripsi (whisper.cpp) →
**deteksi loop halusinasi** → **koreksi transkrip (LLM)** → segmentasi kandidat →
**LLM MEMILIH NOMOR kandidat** (bukan mengarang timestamp — lihat notes/18) +
scoring → buang kandidat dari cuplikan pembuka → render klip 9:16 + burn .ass.

**Watermark** (gambar PNG milik pengguna + teks headline di atasnya) menumpang
dua jalur yang sudah ada, bukan jalur ketiga: gambarnya satu rantai `movie=` di
dalam `-vf` (`ffmpeg.watermarkChain`, ikut kepakai `/api/frame`), teksnya satu Style
+ baris statis di .ass yang sama (`subtitle.writeHeadline`). Karena burn subtitle
adalah filter TERAKHIR, teks otomatis mendarat di atas gambar. **Hanya ikut ke
berkas yang subtitle-nya dibakar** — berkas `clean` tetap bersih. Sumber teks
`llm` memakai `llm.Pick.Title` klip itu, jadi tak ada panggilan LLM tambahan.

Ukuran gambarnya sebuah **KOTAK** (`-watermark-width` x `-watermark-height`,
persen sisi bingkai, bawaan 25x25) dan gambarnya dimuat UTUH ke dalamnya:
`force_original_aspect_ratio=decrease` di ffmpeg, `object-fit: contain` di
pratinjau. Tidak digepengkan, tidak dipotong.

Setiap job diakhiri **ringkasan waktu per tahap** (tabel monospace di terminal &
kotak log GUI), lengkap dengan rasio realtime. Itu angka pembanding antar
percobaan — model, mesin skor, CPU vs GPU.

Tiap klip juga menghasilkan `<clip>.txt`: ucapan klip itu tanpa timestamp dan
tanpa nomor, satu kalimat per baris. Bukan pengganti `.srt` (yang untuk editor
video dan hanya ada di mode clean/both) — berkas ini untuk ditempel ke LLM mana
pun saat membuat caption, jadi selalu ditulis apa pun mode simpannya.

## Peta paket engine

| Paket           | Isi                                                                           |
| --------------- | ----------------------------------------------------------------------------- |
| config          | Options, Layout (folder), ResolvePaths                                        |
| ffmpeg          | ekstrak audio, clip + zoom/reframe, burn subtitle                             |
| transcribe      | wrapper whisper-cli (-ojf), parse segmen + kata bertimestamp                  |
| audio           | baca WAV (PCM 16-bit) + hitung RMS energi per hop                             |
| segment         | bangun kandidat klip (incar durasi ideal, potong di kalimat/jeda)             |
| score/heuristic | aturan Indonesia (emosi, hook, energi, durasi)                                |
| score/llm       | Claude API (pilih momen, skor + judul + hashtag)                              |
| score/ollama    | LLM lokal via Ollama + klien OpenAI-compatible (OpenAI/Gemini/DeepSeek)       |
| subtitle        | tulis .ass (normal/karaoke/word) & .srt                                       |
| correct         | koreksi transkrip via LLM + penyejajaran ulang timestamp per kata             |
| pipeline        | orkestrasi + progress callback                                                |
| job             | store job in-memory + SSE broadcast                                           |
| api             | HTTP handlers + SSE, pemilih berkas, kunci sesi                               |
| setup           | status komponen + unduh/pasang whisper.cpp, ffmpeg, model                     |
| httpx           | satu perilaku ulang-coba (429/5xx) untuk semua panggilan LLM                   |
| capture         | foto layar halaman web via Chrome headless (exec, + terjemah path WSL)        |
| news            | RSS + metadata artikel (Open Graph) + ekstraksi paragraf + pemilih hook (LLM) |
| news/google.go  | membuka pengalih news.google.com lewat RPC-nya sendiri (tanpa browser)        |
| card            | kartu berita: data artikel → template HTML → PNG 1080x1920 + caption/sumber   |
| writer          | pembuat berita: fakta per artikel → satu artikel + pagar fakta (notes/38)     |
| caption         | caption memancing dari ucapan video (notes/40); transkrip + koreksi dipakai ulang |
| watermark       | bakar gambar + headline ke video yang sudah 9:16 (notes/41); tanpa transkripsi  |

## Folder data: dua bentuk, dipilih sendiri

`config.Locate` menentukan di mana engine menulis, dan tidak ada flag untuk
memilihnya — penandanya keberadaan `engine/go.mod`:

- **checkout sumber** → `<repo>/data`, `<repo>/models`, `<repo>/bin`,
  `<repo>/.env`. Persis seperti sebelumnya, jadi model & cache yang sudah ada
  tetap terpakai.
- **terpasang** → folder data per pengguna (`%LOCALAPPDATA%\Clipper` ·
  `~/Library/Application Support/Clipper` · `$XDG_DATA_HOME/clipper`). Wajib,
  sebab `Program Files` dan `/Applications` hanya-baca.

Font selalu dicari di `assets/fonts` **di sebelah biner** (ia dibundel, tidak
pernah berubah). Jangan menulis apa pun ke sana.

Akibatnya untuk kode baru: **jangan pernah menyusun path dari root proyek.**
Pakai `Paths.DataDir` / `ModelsDir` / `ToolsDir` / `EnvFile`. Timpaan env
(`CLIPPER_DATA_DIR`, `CLIPPER_MODELS_DIR`, `CLIPPER_TOOLS_DIR`,
`CLIPPER_FONTS_DIR`, `CLIPPER_ENV_FILE`) menang atas keduanya. Lihat
`notes/23-aplikasi-desktop.md`.

## Desktop: tiga hal yang sudah berlaku

Rinciannya di `notes/23`–`26`; yang wajib diingat saat menulis kode baru:

- **Berkas lokal tidak diunggah.** GUI punya pemilih berkas sendiri lewat
  `/api/browse`, dan berkas yang di-drop dicari di tempatnya lewat `/api/locate`.
  Jangan menambah alur yang menyalin video ke `data/uploads` (notes/24).
- **Komponen dipasang engine.** `internal/setup` mengunduh whisper.cpp, ffmpeg,
  dan model ke `ToolsDir`/`ModelsDir`; halaman Requirements memanggilnya. Kalau
  menambah komponen, tambahkan resepnya di sana — bukan instruksi di README
  (notes/25).
- **GUI tidak mengendalikan apa pun yang memakan waktu.** Pemasangan komponen
  dan job klip berjalan di latar dengan `context.Background()`; GUI memulai lalu
  BERLANGGANAN kabarnya lewat SSE. Pernah dilanggar sekali (unduhan hidup di
  dalam `r.Context()` permintaan halaman) dan akibatnya unduhan 111 MB mengulang
  dari nol setiap pengguna pindah jendela.
- **GUI disajikan engine.** `next build` menghasilkan `gui/out` (ekspor statis),
  dan engine menyajikannya di akar alamatnya. `npm run dev` tetap ada untuk
  mengembangkan, tapi aplikasi jadi tidak memakainya. Jendela Tauri di
  `desktop/` cuma membuka satu alamat yang dicetak `clipper serve -shell`
  (notes/27).
- **Kunci sesi.** Saat terpasang, engine memakai port acak + kunci yang wajib
  ada di setiap permintaan, ditulis ke `<DataDir>/engine.json`. Kuncinya tinggal
  di cookie `HttpOnly` yang dipasang engine saat halaman pertama dibuka; `/api/`
  **menolak** `?token=` (notes/30). Di GUI, SEMUA URL engine tetap dibentuk lewat
  `eng()` di `gui/app/engine.ts` — jangan menyusun `${ENGINE}/api/...` sendiri,
  sebab alamat engine baru diketahui saat halaman dibuka (portnya acak).
- **Unduhan wajib dipaku sidik jarinya.** Tiap alamat di `internal/setup` punya
  sha256; sumber tanpa itu ditolak sebelum diunduh. Menaikkan `whisperVersion`,
  `ffmpegVersion`, `btbnBuild`, atau `modelRevision` berarti menghitung ulang
  sha256-nya — satu paket, jangan dipisah (notes/30).
- **POST ke engine wajib `application/json`**, dan engine hanya menjawab
  alamatnya sendiri. Penjaganya di `internal/api/guard.go`; path yang diteruskan
  ke ffmpeg lewat `ffmpeg.CLIPath` (notes/30).

## Mesin skor

Satu daftar mesin untuk SELURUH aplikasi (`internal/api/engines.go`, notes/39):
Ollama lokal, Claude, ChatGPT, Gemini, DeepSeek, **Custom (OpenAI-compatible)**
— plus `heuristic` (tanpa LLM) khusus halaman klip. `custom` adalah gateway
milik pengguna: satu-satunya mesin tanpa alamat & model bawaan, sebab keduanya
memang cuma dia yang tahu. Alamat yang disimpan SELALU dipangkas akhiran `/v1`
(`normalizeBase`) — dokumentasi penyedia menuliskannya, klien menambahkannya
sendiri, dan gabungannya jadi `/v1/v1/messages`. Kunci API diisi sekali di
halaman Requirements; mesin & model dipilih tiap kali bekerja lewat
`<EnginePicker>` yang sama di semua tab.

Semua panggilan LLM lewat `internal/httpx.Retry`: 3x percobaan dengan jeda
berlipat, HANYA untuk 429/500/502/503/504, menghormati `Retry-After`. Bukan
setelan dan jangan dijadikan setelan — satu 429 di panggilan ke sekian membunuh
job yang whispernya sudah dibayar, dan angka yang bisa diketik cuma menambah
cara untuk salah. 400/401/404 tidak pernah diulang: itu salah konfigurasi.

`-mode offline|hybrid` **dibuang 18 Agustus 2026**: ia tinggal menentukan nilai
bawaan `-provider` dan mesin koreksi transkrip, dan keduanya kini dijawab
pemilih mesin. Setelan yang cuma menentukan bawaan setelan lain adalah satu cara
lagi untuk tidak sinkron.

Mode `online` dan reframe `face_follow` **dibuang 5 Agustus 2026**: keduanya cuma
nama tanpa isi — `online` berperilaku persis sama dengan hybrid, `face_follow`
selalu ditolak. Kalau salah satunya dikerjakan nanti, tambahkan lagi bersama
implementasinya, bukan sebelumnya (notes/07).

**Tanpa fallback**: mesin yang dipilih pengguna dipakai apa adanya. Bila gagal,
job berhenti dengan pesan akar masalah — engine TIDAK diam-diam pindah ke
heuristik. Lihat `notes/12-kebijakan-mesin-skor.md`.

## Transkripsi: flag yang TIDAK boleh diubah sembarangan

whisper dipanggil dengan **`-mc 0`** (jangan suapkan teks sebelumnya sebagai
konteks) dan **`-sns`**. Tanpa `-mc 0`, model besar bisa terjebak mengulang satu
kalimat sampai audio habis — pernah terjadi: 1603 dari 1644 segmen isinya
kalimat yang sama.

Konsekuensinya: **`--prompt` whisper mati total** saat `-mc 0`, jadi kosakata
tidak bisa dibias di tingkat decoding. Itu sebabnya nama & istilah daerah
dibenahi di tahap koreksi lewat `-terms`. Jangan mencoba menghidupkan `--prompt`
tanpa membaca `notes/21-flag-whisper.md` lebih dulu.

Flag decoding ikut menentukan kunci cache: **naikkan awalan versi di
`transcriptCacheKey`** (kini `v2`) setiap kali flag berubah, kalau tidak
transkrip lama dipakai ulang dan perubahannya seolah tak berefek.

Ada penjaga `detectRepetitionLoop` sebelum tahap koreksi: job berhenti dengan
pesan yang menyebut kalimat pengulangnya, bukan diam-diam menghasilkan klip
sampah.

## Koreksi transkrip

Keluaran mentah whisper membawa tanda hubung dialog, tanda baca salah tempat, dan
kata salah dengar. Semuanya ikut terbakar ke subtitle DAN menyesatkan segmentasi
(`BuildCandidates` memotong di akhir kalimat). Karena itu transkrip dikoreksi LLM
lebih dulu — menyala secara default, matikan dengan `-transcript-fix off`.

Butuh LLM walau mesin skornya heuristik: memakai mesin yang sama dengan
pemilihan momen, dan bila mesin itu `heuristic` ia memakai Ollama. Bila tak
terjangkau, job berhenti dengan pesan sebabnya.

Paket `correct` MENGOREKSI, bukan menulis ulang: tiap segmen dijaga empat pagar
deterministik (panjang, jatah kata isi yang berubah, tanda kutip hilang, balasan
kosong), dan koreksi yang ditolak dilaporkan di `Report.Rejected`. Timestamp per
kata disejajarkan ulang lewat jarak edit supaya mode karaoke/word tetap tepat.
Hasilnya di-cache di `data/cache/corrected`. Lihat `notes/14-koreksi-transkrip.md`.

**Daftar istilah (`-terms`)** memperbaiki nama & kata daerah yang salah dengar
("Londo Irang" → "Londo Ireng"). Diperlukan karena `--prompt` whisper mati (lihat
di atas), DAN karena prompt koreksi justru melarang menyentuh kata daerah —
daftar ini pengecualiannya. Ikut jadi bahan kunci cache koreksi, jadi menambah
istilah memicu koreksi ulang. Untuk tugas ini **llama3.1 jelas lebih baik
daripada qwen2.5**. Lihat `notes/22-daftar-istilah.md`.

Transkrip panjang dipecah bertumpang-tindih sebelum dikirim ke LLM (12 mnt
Ollama / 25 mnt Claude); momen yang terbelah batas disambung lewat tanda
`"continues"`. Transkrip di-cache di `data/cache/transcripts` (kunci = sidik
jari isi video + model + bahasa).

## Sasaran: DESKTOP. Titik.

Ditegaskan ulang 6 Agustus 2026 setelah sempat saya longgarkan. **Web bukan
sasaran, bukan sasaran cadangan, dan bukan "nanti mungkin".**

> **Pengecualian: branch `webview` SAJA** (diputuskan pemilik 8 Oktober 2026).
> Di branch ini ada `clipper serve -web` untuk tim kecil dengan satu kata sandi
> bersama di VPS — lihat `notes/42-versi-web.md`. Jangan digabung ke `main`
> tanpa keputusan pemilik; di `main` aturan di bawah tetap berlaku utuh.

Yang boleh: **membuka GUI lewat browser sebagai ALAT UKUR** — itu yang
memungkinkan `scripts/measure-ui.mjs` memotret dan mengukur tinggi kolom lewat
Chrome headless. Itu perkakas pengembangan, bukan bentuk produk.

Yang TIDAK boleh, dan jangan ditawarkan lagi:

- deploy ke server, hosting, atau alamat publik;
- multi-pengguna, akun antar-pengguna, autentikasi antar-pengguna;
- responsif untuk ponsel/tablet;
- keputusan tampilan yang mengorbankan jendela desktop demi tab browser.

Ukuran acuan tetap **900×600** (batas terkecil `tauri.conf.json`) dan
**1240×860** (bawaan) — bukan lebar layar penuh, bukan lebar ponsel.

Kunci sesi di `notes/26`/`30` dirancang untuk **satu mesin, satu pengguna**.
Jangan dilebarkan tanpa membaca ulang kedua catatan itu.

## Tampilan: dua aturan yang TIDAK bisa ditawar

Keduanya sudah diminta berkali-kali dan tetap terlanggar, sebab dulu hanya
tertulis di `notes/29` — berkas 700 baris yang tidak dibaca ulang tiap sesi.
Sekarang di sini, dan berlaku untuk SETIAP perubahan tampilan.

### 1. Jendela tidak boleh bergulir. Titik.

> **Pengecualian: halaman Video clips (`/`) dan News cards (`/news`)** —
> keputusan pemilik 9 Oktober 2026, `DESIGN.md` ("Video Clips") dan
> `DESIGN-NEWSCARD.md`. Keduanya tiga layar (klip: Atur klip → Sedang
> diproses → Hasil; kartu: Pilih artikel → Susun kartu → Simpan & bagikan)
> di dalam `.screen.scroll` yang BOLEH bergulir
> ke bawah; kepala halaman dan pratinjau menempel (`position: sticky`). Rail
> kiri tetap. Halaman lain tetap tunduk pada aturan di bawah ini, dan
> `measure-ui` mencatat kedua halaman itu sebagai `-`.

Yang boleh bergulir hanya **kotak yang memang daftar**: kotak log, daftar
berita, daftar paragraf. Selain itu — kolom setelan, panel, halaman — harus
MUAT. Kalau tidak muat, yang dibuang isinya, bukan syaratnya.

**Dan sekarang bisa DIUKUR, jadi jangan lagi menyerahkan tebakan.**

```bash
./bin/clipper serve                                  # terminal lain
~/.cache/ms-playwright/chromium-*/chrome-linux64/chrome \
  --headless --remote-debugging-port=9333 --user-data-dir=/tmp/cdp about:blank &
node scripts/measure-ui.mjs http://127.0.0.1:8787 /tmp/shots
```

Ia mencetak `scrollHeight` vs `clientHeight` tiap kolom di 900×600 dan
1240×860, tema terang dan gelap, sekaligus memotret tiap halaman. Angka `over`

> 0 berarti kotak itu bergulir. **Baseline 17 Agustus 2026: klip, kartu berita,
> dan pembuat berita `0/0` di 1240×860.** Kalau angkanya naik lagi, perubahanmu
> yang menaikkannya.

Potretnya juga wajib dilihat, bukan cuma angkanya: kotak kosong, teks terpotong,
dan lambang yang tidak ada di font hanya ketahuan dari gambar. `ⓘ` (U+24D8)
lolos dari semua grep dan tampil sebagai kotak kosong selama berbulan-bulan —
ketahuan pada potret pertama.

Sebelum menyerahkan perubahan tampilan apa pun, periksa:

- tidak ada panel yang tingginya bergantung pada isi yang bisa tumbuh;
- tiap daftar punya `max-height` + `overflow-y: auto`;
- kendali yang cuma satu angka memakai `<Stepper>`, bukan `<input type="range">`
  yang melar selebar kolomnya;
- tidak ada kalimat pengantar atau keterangan yang mengulang apa yang sudah
  terlihat — SELAIN satu judul + satu baris subjudul di `.page-header`
  (keputusan pemilik 9 Oktober 2026, `DESIGN.md` §5.2).

### 2. Halaman `/` (Video clips) adalah STANDAR. Salin, jangan mengarang.

Halaman lain mengikuti bentuknya; jangan menemukan tata letak baru tiap halaman.
Bentuk bakunya:

**Tidak ada bilah ikon di atas.** Navigasi kiri mengikuti DESIGN-Navigasi
(bagian kedua `DEsign-clipper-lanjutan.md`): kelompok BUAT + KELOLA (Settings
kini item menu), kaki rail = kartu pekerjaan berjalan + tombol status mesin AI
+ baris akun (avatar, tema; bahasa di popup akun). ≥1100px sidebar 220px,
di bawahnya rail ikon 72px — 900x600 hanya punya 505 px tinggi, jadi ukur ulang
bila menambah item.

**Satu `.page-header` per halaman** (keputusan pemilik 9 Oktober 2026,
`DESIGN.md` §5.2): `<h1>` judul halaman, satu baris subjudul, status job, dan
SATU aksi utama. Ia menggantikan `.start-panel` — tombol jalan tidak ada di dua
tempat. Tinggi yang ia ambil dibayar dari isi halaman, jadi ukur ulang.
Arah rupa (warna, huruf, radius, elevasi) ada di `DESIGN.md`; bagian
responsifnya (§7) DIBUANG, acuan tetap 900×600 dan 1240×860.

```
.screen
├── .page-header          <h1> + subjudul + status job + aksi utama
├── .screen-head          hanya bila ada galat/peringatan
└── .screen-body.two
    ├── .screen-main      KIRI: yang dilihat
    │   └── .panel > .sub-layout
    │        ├── .sub-preview    bingkai pratinjau
    │        └── .sub-settings   setelan yang MENGUBAH pratinjau itu
    │                            (kelompok bernama, tiap kelompok .grid3)
    └── .screen-col       KANAN: yang diisi & dipilih
        └── .panel        kelompok bernama, dibaca atas ke bawah
```

Aturan pembagian kolomnya satu kalimat: **kiri = pratinjau + apa pun yang
mengubah rupanya; kanan = masukan dan pilihan; tombol jalan di `.page-header`.** Setelan rupa
TIDAK pernah ditaruh di kolom kanan, dan isian sumber TIDAK pernah ditempel ke
panel pratinjau.

Label = NAMA (satu-dua kata). Penjelasan pindah ke `title` atau ke teks
pilihannya. Rinciannya di `notes/29`.

## Konvensi

- **Komentar** kode dalam **bahasa Indonesia**.
- **Semua yang lain dalam bahasa Inggris**: nama folder, berkas, paket, fungsi,
  variabel, konstanta, nama test, field JSON API, nama event SSE, nilai flag CLI,
  dan seluruh pesan yang dilihat pengguna.
- **Jangan `window.confirm/alert/prompt`** — kotaknya milik browser ("localhost
  says…"), tidak ikut tema. Konfirmasi lewat `useConfirm()` (`gui/app/confirm.tsx`).
- Teks antarmuka GUI lewat kamus `gui/app/i18n.tsx` (EN sumber kebenaran; kunci
  bahasa Inggris wajib punya pasangan Indonesia, dijaga oleh TypeScript).
- Teks yang engine tulis ke kartu (tanggal, kaki kartu, berkas pendamping)
  mengikuti parameter `lang` dari klien; default `en`.
- Biner eksternal & model besar TIDAK di-commit (lihat .gitignore).
- Model whisper default engine: `small` (produksi). Uji cepat: `base`.
- Video besar: audio di-stream via ffmpeg (tidak dimuat ke RAM).
- TGanti Teks Warna Merah untuk task yang belum dikerjakan
- Ganti Teks Warna Kunin untuk task yang perlu keputusan dari pemilik proyek

## Cara mengarahkan pemilik proyek (mode belajar)

Pemilik proyek mengerjakan sendiri perubahan kodenya dan memakai agen sebagai
pengarah, bukan pengetik. Kalau ia minta "arahkan", "aku mau belajar", atau
menyebut "jangan rubah code": **jangan menyunting berkas apa pun** — termasuk
setelah beberapa giliran berlalu; larangan itu berlaku sampai ia mencabutnya.

Bentuk arahan yang dipakai — **satu bagian per suntingan**, urut dari baris
paling bawah ke paling atas supaya nomor baris tidak bergeser saat ada baris
yang dihapus:

1. nomor baris
2. blok kode **Dari:**
3. blok kode **Jadi:** (atau "hapus seluruh barisnya")
4. **Kenapa:** 1–3 kalimat — alasannya, bukan sekadar apa yang berubah

Tutup dengan perintah verifikasi (build/vet/test/grep) dan angka yang harus
muncul kalau benar. Sebutkan angka harapan itu **sebelum** perintahnya
dijalankan; itu yang membedakan "kelihatan jalan" dari "terbukti jalan".

Yang bikin arahan gagal, dari pengalaman sesi pemindahan worker C++ ke Go:

- **penjelasan konsep panjang dulu, daftar suntingan belakangan.** Sertakan
  alasan di dalam tiap suntingan, jangan dipisah jadi esai tersendiri.
- **memberi banyak langkah sekaligus saat ia bilang bingung.** Turunkan jadi
  satu suntingan, minta hasil build, baru lanjut.
- **kalimat yang bisa ditafsir lebih luas dari maksudnya.** "`ctx` hilang dari
  pemanggilan" terbaca sebagai "buang `ctx` dari tanda tangan fungsi" dan
  memecahkan tujuh tempat lain. Sebut ruang lingkupnya dengan tegas.

Kalau ia melaporkan error, **baca keadaan berkasnya lebih dulu** (build, diff,
baris yang disebut) sebelum menebak sebabnya — dua kali dalam sesi itu tebakan
meleset karena suntingannya ternyata belum sesuai yang dikira.

## Status: MVP JALAN

CLI + HTTP + GUI semua berfungsi (mode offline diverifikasi
end-to-end). Lihat notes/08-status-mvp.md untuk yang sudah/belum.

```

```
