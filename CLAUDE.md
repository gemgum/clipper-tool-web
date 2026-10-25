# CLAUDE.md — Panduan untuk agen/sesi berikutnya

Proyek **Clipper**: alat kerja tim konten, versi **web** (engine di VPS, dipakai
lewat browser). Inti awalnya memotong video panjang (1–4 jam) jadi klip pendek
9:16 bersubtitle otomatis + skor "berpotensi viral". Konten target: **bahasa
Indonesia**.

## Arsitektur

```
browser --HTTPS--> Caddy/nginx --> engine/ (Go)  --exec-->  whisper.cpp, ffmpeg, Chrome
                                     ^     |
                   gui/ (Next.js) ---+     +-- menyajikan gui/out (statis)
                   HTTP + SSE
```

- **engine/** (Go) — HTTP API, pipeline, scoring, render, pemasangan komponen,
  dan penyaji antarmuka. **Pustaka standar saja** (tanpa dependency eksternal).
- **gui/** (Next.js) — dibangun jadi berkas statis `gui/out` yang disajikan
  engine. Tab: `/` Klip (form → progress SSE → hasil), `/news` kartu berita
  (`notes/13`, crawl `notes/45`), `/writer` pembuat berita (`notes/38`),
  `/captions` caption (`notes/40`), `/watermark` Mark: logo + judul ke Shorts
  9:16 / feed 4:5 / 1:1, foto juga (`notes/41`, `notes/44`), `/history`, dan
  `/requirements` (status komponen, mesin AI, kunci). Bahasa antarmuka EN/ID
  (default EN, localStorage).
- **deploy/** — unit systemd + Caddyfile/nginx untuk VPS (`notes/42`).
- **notes/** — keputusan desain & alasannya, dirujuk dari komentar kode.
- Worker C++ dan aplikasi desktop (Tauri) sudah dibuang (`notes/20`); jangan
  dikembalikan.

## Perintah penting

```bash
./setup.sh [base|small|medium|large-v3|large-v3-turbo]   # whisper.cpp + model + font
./fetch-fonts.sh                 # font bundel (juga dipanggil setup.sh; jalankan di VPS saat font bertambah)
./build.sh                       # engine (bin/clipper) + gui statis (gui/out)
./bin/clipper help               # semua perintah & flag (run, write, caption, watermark, fetch, serve)
./bin/clipper serve              # API + GUI di 127.0.0.1:8787; banner mencetak satu alamat "open:"
./bin/clipper serve -web -host <nama>   # versi web: login sandi/SSO, berkas diunggah (notes/42)
cd gui && npm run dev            # GUI mode pengembangan di 3000 (hot reload)
```

Deploy ke VPS (layanan systemd `clipper`, susunan folder di `notes/42`): biner STATIS
(`CGO_ENABLED=0 go build -ldflags "-X main.version=$(git describe --always --dirty)"`,
glibc VPS lebih tua), salin ke `/var/www/clipper/bin/clipper` (simpan dulu
`clipper.prev`), `rsync --delete gui/out/` ke `/var/www/clipper/gui/out/`, font
baru ke `/var/www/clipper/assets/fonts/`, lalu `systemctl restart clipper`.
VPS: 2 CPU, 4 GB RAM, `PrivateTmp=yes`.

Kartu berita butuh Chrome/Chromium; engine mencarinya sendiri, timpa dengan
`CLIPPER_CHROME=/path/ke/chrome`.

## Alur pipeline klip (engine/internal/pipeline)

extract audio (ffmpeg) → RMS energi (paket `audio`) → transkripsi (whisper.cpp)
→ **deteksi loop halusinasi** → **koreksi transkrip (LLM)** → segmentasi
kandidat → **LLM MEMILIH NOMOR kandidat** (bukan mengarang timestamp —
`notes/18`) + scoring → buang kandidat dari cuplikan pembuka (`notes/17`) →
render klip 9:16 + burn .ass.

Setiap job diakhiri **ringkasan waktu per tahap** (tabel monospace di terminal &
log GUI) dengan rasio realtime — angka pembanding antar model/mesin/CPU.

Tiap klip juga menghasilkan `<clip>.txt`: ucapan klip tanpa timestamp, satu
kalimat per baris, untuk ditempel ke LLM saat membuat caption. Selalu ditulis,
apa pun mode simpannya (`.srt` hanya di mode clean/both).

**Subtitle** (`subtitle.WriteASS`): mode normal | highlight | karaoke | word |
build; gerak kata aktif (pop/bounce/rise), bayangan (soft/hard/none), satu kata
per baris, warna berselang, huruf kapital. Preset gaya caption ada di
`gui/app/caption-presets.tsx`; angka geraknya sama dengan `subtitle.animTag`.
Font subtitle = `api.fontCatalog` (berkas statis, family internalnya harus cocok
dengan nama yang ditulis ke .ass — font variabel tidak dikenali libass).

## Mark (logo + judul) — satu jalur untuk Klip, Mark, dan News

Gambar = rantai `movie=` di dalam `-vf` (`ffmpeg.watermarkChain`, juga dipakai
`/api/frame`); teks = satu Style + baris statis di .ass yang sama
(`subtitle.writeHeadline`). Burn subtitle adalah filter TERAKHIR, jadi teks
mendarat di atas gambar. **Hanya ikut ke berkas yang subtitle-nya dibakar**
(`clean` tetap bersih). Sumber judul `llm` memakai `llm.Pick.Title` klip itu
(tanpa panggilan LLM tambahan). Klip dan Mark memasang lapisan lewat
`watermark.ApplyLayers`.

Ukuran gambar = **KOTAK** (persen sisi bingkai, bawaan 25×25) dan gambarnya
dimuat UTUH di dalamnya (`force_original_aspect_ratio=decrease` / `object-fit:
contain`). Tidak digepengkan, tidak dipotong. Subtitle Klip otomatis
menghindari kotak judul (`useMark().hlBand`).

**News memakai Mark yang sama** (keputusan pemilik 24 Oktober 2026): tiap kartu
wajib membawa mark. `/api/card` menerima `mark` dan `layout`: `card` = PNG kartu
dari Chrome lalu dibakar Mark, `box` = foto artikel penuh + kotak judul Mark
(tanpa Chrome). Keduanya lewat `watermark.BurnTo`. Template Mark satu daftar
untuk kedua halaman; satu bisa ditandai bawaan
(`POST /api/watermark/templates/{name}/default`).

## Kartu berita: pratinjau & render

Di tab itu LLM **boleh menulis** ringkasan, caption, dan teks carousel
(keputusan pemilik 9 Oktober 2026, `DESIGN-NEWSCARD.md`), TAPI setiap tulisannya
diperiksa pagar fakta `writer.CheckText`: angka, kutipan, dan nama yang tidak
ada di artikel ditandai di bawah isiannya.

- **Mini preview** = `/api/card/html` (template yang SAMA dengan PNG) di iframe
  + `MarkLayers`. Font tertanam (±1,25 MB) TIDAK ikut di HTML: diganti penanda
  `api.CardFontMark`, CSS-nya dari `GET /api/card/fonts?v=<kunci>` yang disimpan
  cache browser. Halaman yang sudah dibuat disimpan per setelan di GUI.
- **Foto artikel** tampil lewat `/api/news/img?url=&w=240|1080` (salinan kecil
  di `data/newsimg`, disapu 7 hari) — satu-satunya pembentuk alamat foto di GUI
  adalah `photoURL` (news/page.tsx). Chrome memakai salinan yang sama
  (`renderImage`), bukan mengunduh ulang dari situs berita. Pengecualian: foto
  di dalam iframe mini preview masih dari situs berita (iframe tanpa cookie
  login).
- **Render PNG** (`card.BuildPages`): seluruh carousel — termasuk kartu pertama —
  ditumpuk sebagai iframe dalam SATU lembar, dipotret Chrome sekali, lalu
  dipotong per kartu (`shootSheet`/`splitSheet`; font sekali dari berkas
  sementara). Menyalakan Chrome adalah ongkos terbesar (±1,7 dtk per kartu di
  VPS). Hasil identik piksel dengan render satu per satu. Logo Mark di halaman
  lanjutan dibakar paralel sebanyak CPU.
- **Pratinjau hasil**: halaman yang dilihat dirender dulu (folder
  `card-preview-pN`), lalu seluruh carousel dalam satu permintaan; di layar
  Susun kartu seluruh carousel juga dirender otomatis setelah setelan diam 3
  detik. Gambar dilihat sebagai JPEG (`?jpg=1`); unduhan tetap PNG. Render
  pratinjau bersamaan = separuh CPU, 1..3 (`previewSlots`, dikirim ke GUI
  sebagai `preview_slots`) — di VPS 2 CPU render paralel hanya memperlambat
  halaman yang dilihat (diukur: total sama, tiap halaman 2–3× lebih lambat).

## Peta paket engine

| Paket | Isi |
| --- | --- |
| config | Options, Layout (folder), ResolvePaths, konstanta bersama (NewsBox*, CardPhoto*, FrameSize) |
| ffmpeg | ekstrak audio, clip + zoom/reframe, burn subtitle, Thumb, ExtractFrame |
| transcribe | whisper-cli (-ojf), AI Studio, OpenAI-compatible; segmen + kata bertimestamp |
| audio | baca WAV (PCM 16-bit) + RMS energi per hop |
| segment | kandidat klip (incar durasi ideal, potong di kalimat/jeda) |
| score/heuristic | aturan Indonesia (emosi, hook, energi, durasi) |
| score/llm | Claude (pilih momen, skor + judul + hashtag) |
| score/ollama | Ollama lokal + klien OpenAI-compatible (OpenAI/Gemini/DeepSeek/custom) |
| subtitle | .ass (semua mode subtitle + judul Mark) & .srt |
| fontmetrics | lebar teks dari berkas font yang sama dengan libass |
| correct | koreksi transkrip via LLM + penyejajaran ulang timestamp per kata |
| pipeline | orkestrasi + progress callback + cache transkrip |
| job | store job + SSE broadcast |
| api | HTTP handlers + SSE, pemilih berkas, kunci sesi, mode web |
| setup | status komponen + unduh/pasang whisper.cpp, ffmpeg, model (sha256 dipaku) |
| httpx | ulang-coba LLM (429/5xx) + GetImage |
| textx | ExtractJSON, Snip — pembantu teks bersama |
| capture | foto layar via Chrome headless (+ terjemah path WSL) |
| news | RSS, metadata artikel, ekstraksi paragraf, pemilih hook, crawl (Serper/SociaVault) |
| card | kartu berita: HTML template → PNG (lembar satu kali Chrome) + caption/sumber |
| writer | pembuat berita: fakta per artikel → satu artikel + pagar fakta |
| caption | caption dari ucapan video; transkrip + koreksi dipakai ulang |
| watermark | bakar Mark ke video/foto yang sudah jadi; tanpa transkripsi |
| ytdl | unduh video YouTube (yt-dlp, lalu Cobalt) |
| types | tipe bersama (segmen, kata) |

## Folder data: dua bentuk, dipilih sendiri

`config.Locate` menentukan di mana engine menulis (tanpa flag) — penandanya
keberadaan `engine/go.mod`:

- **checkout sumber** → `<repo>/data`, `<repo>/models`, `<repo>/bin`, `<repo>/.env`.
- **terpasang** → folder data per pengguna (`$XDG_DATA_HOME/clipper` dan
  padanannya). Di VPS semuanya ditimpa env (`notes/42`).

Font selalu dicari di `assets/fonts` **di sebelah biner** (dibundel, tidak pernah
berubah); jangan menulis apa pun ke sana.

Kode baru: **jangan pernah menyusun path dari root proyek.** Pakai
`Paths.DataDir` / `ModelsDir` / `ToolsDir` / `EnvFile`. Timpaan env
(`CLIPPER_DATA_DIR`, `CLIPPER_MODELS_DIR`, `CLIPPER_TOOLS_DIR`,
`CLIPPER_FONTS_DIR`, `CLIPPER_ENV_FILE`) menang atas keduanya.

**Berkas sementara wajib dibersihkan.** Pola yang dipakai: tulis ke nama
sementara lalu ganti nama, dan buang nama sementaranya bila gagal
(`thumbFile`, `writeFileAtomic`); folder kerja dengan `defer os.RemoveAll`.
Cache berumur: `newsimg` & `thumbs` 7 hari, folder pratinjau kartu 24 jam,
unggahan yang tak pernah dipakai 48 jam, `job_*/tmp` dibuang saat job selesai
dan saat engine mulai. Yang belum punya batas umur: hasil klip `job_*` (tetap
ada walau job dihapus dari History), unggahan milik job gagal, dan cache
transkrip.

## Mode lokal (`serve` tanpa `-web`)

Rinciannya di `notes/24`–`26`; yang wajib diingat saat menulis kode baru:

- **Berkas lokal tidak diunggah.** GUI punya pemilih berkas sendiri
  (`/api/browse`), berkas yang di-drop dicari di tempatnya (`/api/locate`).
  Jangan menambah alur yang menyalin video ke `data/uploads` (`notes/24`).
- **Komponen dipasang engine** (`internal/setup` → `ToolsDir`/`ModelsDir`,
  halaman Requirements). Komponen baru = resep di sana, bukan instruksi README
  (`notes/25`).
- **GUI tidak mengendalikan apa pun yang memakan waktu.** Pemasangan dan job
  berjalan di latar dengan `context.Background()`; GUI memulai lalu
  BERLANGGANAN kabarnya lewat SSE. Pernah dilanggar sekali (unduhan hidup di
  `r.Context()`) dan unduhan 111 MB mengulang dari nol tiap pindah jendela.
- **Posisi halaman diingat browser, bukan server.** Layar & tab memakai
  `useSticky`, posisi gulir `ScrollKeep` (`persist.ts`). State navigasi baru =
  `useSticky`, bukan `useState`.
- **Kunci sesi.** Saat terpasang: port acak + kunci wajib di setiap permintaan
  (`<DataDir>/engine.json`), disimpan di cookie `HttpOnly`; `/api/` **menolak**
  `?token=` (`notes/30`). Di GUI SEMUA URL engine dibentuk lewat `eng()`
  (`gui/app/engine.ts`) — jangan menyusun `${ENGINE}/api/...` sendiri.
- **Unduhan wajib dipaku sidik jarinya.** Tiap alamat di `internal/setup` punya
  sha256; menaikkan `whisperVersion`, `ffmpegVersion`, `btbnBuild`, atau
  `modelRevision` berarti menghitung ulang sha256-nya dalam satu paket
  (`notes/30`).
- **POST ke engine wajib `application/json`**, dan engine hanya menjawab
  alamatnya sendiri (`internal/api/guard.go`); path ke ffmpeg lewat
  `ffmpeg.CLIPath`.

## Sasaran: WEB

Repo ini versi web saja (dipisah dari `gemgum/clipper-tool` branch `webview`,
Oktober 2026): `clipper serve -web` untuk tim kecil dengan sandi bersama atau
SSO (`notes/42`). Kunci sesi di `notes/26`/`30` dirancang untuk **satu mesin,
satu pengguna**; mode web menurunkannya dari sandi (`api/web.go`). Jangan
dilebarkan tanpa membaca ulang ketiga catatan itu.

## Mesin AI

Satu daftar mesin untuk SELURUH aplikasi (`internal/api/engines.go`,
`notes/39`): Ollama lokal, Claude, ChatGPT, Gemini, DeepSeek, **Custom
(OpenAI-compatible)** — plus `heuristic` (tanpa LLM) khusus halaman klip.
`custom` = gateway milik pengguna, satu-satunya tanpa alamat & model bawaan.
Alamat disimpan **apa adanya**, utuh dengan jalurnya seperti dokumentasi
penyedia (`https://openrouter.ai/api/v1`); klien hanya menambah nama endpoint
(`/chat/completions`, `/messages`, `/audio/transcriptions`). Alamat yang memuat
nama endpoint ditolak saat disimpan; server LOKAL tetap host:port. `.env` lama
dimigrasi sekali oleh `migrateBaseURLs` (penanda `CLIPPER_BASE_URLS=full`).
Kunci API diisi sekali di Settings; mesin & model dipilih tiap kali bekerja
lewat `<EnginePicker>` yang sama di semua tab.

Semua panggilan LLM lewat `internal/httpx.Retry`: 3× dengan jeda berlipat,
HANYA untuk 429/500/502/503/504, menghormati `Retry-After`. Bukan setelan dan
jangan dijadikan setelan. 400/401/404 tidak pernah diulang: itu salah
konfigurasi.

**Tanpa fallback**: mesin yang dipilih pengguna dipakai apa adanya. Bila gagal,
job berhenti dengan pesan akar masalah — engine TIDAK diam-diam pindah ke
heuristik (`notes/12`).

Fitur yang cuma nama tanpa isi dibuang, bukan disimpan: `-mode
offline|hybrid` (18 Agustus 2026 — setelan yang cuma menentukan bawaan setelan
lain), mode `online` dan reframe `face_follow` (5 Agustus 2026). Kalau salah
satunya dikerjakan nanti, tambahkan lagi bersama implementasinya.

## Transkripsi: flag yang TIDAK boleh diubah sembarangan

whisper dipanggil dengan **`-mc 0`** (jangan suapkan teks sebelumnya sebagai
konteks) dan **`-sns`**. Tanpa `-mc 0`, model besar bisa terjebak mengulang
satu kalimat sampai audio habis (pernah: 1603 dari 1644 segmen kalimat yang
sama). Akibatnya **`--prompt` whisper mati total**, jadi nama & istilah daerah
dibenahi di tahap koreksi lewat `-terms`. Jangan menghidupkan `--prompt` tanpa
membaca `notes/21`.

Flag decoding ikut menentukan kunci cache: **naikkan awalan versi di
`transcriptCacheKey`** (kini `v2`) setiap kali flag berubah. Penjaga
`detectRepetitionLoop` menghentikan job dengan pesan yang menyebut kalimat
pengulangnya.

## Koreksi transkrip

Menyala secara default (`-transcript-fix off` untuk mematikan): tanda hubung
dialog, tanda baca salah tempat, dan kata salah dengar ikut terbakar ke
subtitle DAN menyesatkan segmentasi. Butuh LLM walau mesin skornya heuristik
(memakai Ollama bila mesin `heuristic`); bila tak terjangkau, job berhenti.

Paket `correct` MENGOREKSI, bukan menulis ulang: empat pagar deterministik
(panjang, jatah kata isi yang berubah, tanda kutip hilang, balasan kosong);
yang ditolak dilaporkan di `Report.Rejected`. Timestamp per kata disejajarkan
ulang lewat jarak edit. Cache di `data/cache/corrected` (`notes/14`).

**Daftar istilah (`-terms`)** memperbaiki nama & kata daerah yang salah dengar
("Londo Irang" → "Londo Ireng") — pengecualian dari larangan menyentuh kata
daerah; ikut jadi kunci cache. Untuk tugas ini **llama3.1 jelas lebih baik
daripada qwen2.5** (`notes/22`).

Transkrip panjang dipecah bertumpang-tindih sebelum dikirim ke LLM (12 mnt
Ollama / 25 mnt Claude); momen yang terbelah disambung lewat `"continues"`.
Transkrip di-cache di `data/cache/transcripts` (kunci = sidik jari isi video +
model + bahasa).

## Tampilan: dua aturan yang TIDAK bisa ditawar

Berlaku untuk SETIAP perubahan tampilan.

### 1. Jendela tidak boleh bergulir. Titik.

> **Pengecualian: halaman Klip (`/`) dan News (`/news`)** — keputusan pemilik 9
> Oktober 2026, `DESIGN.md` dan `DESIGN-NEWSCARD.md`. Keduanya tiga layar (klip:
> Atur klip → Sedang diproses → Hasil; kartu: Pilih artikel → Susun kartu →
> Simpan & bagikan). Rail kiri tetap.

Yang boleh bergulir hanya **kotak yang memang daftar** (log, daftar berita,
daftar paragraf) dan kolom kiri halaman berkolom. Selain itu harus MUAT. Kalau
tidak muat, yang dibuang isinya, bukan syaratnya. Kartu sumber (Klip) / Files
(Mark) menempel di atas kolom kiri pada jendela setinggi ≥ 760 px.

**Ukur, jangan menebak:**

```bash
./bin/clipper serve                                  # terminal lain
~/.cache/ms-playwright/chromium-*/chrome-linux64/chrome \
  --headless --remote-debugging-port=9333 --user-data-dir=/tmp/cdp about:blank &
node scripts/measure-ui.mjs http://127.0.0.1:8787 /tmp/shots
```

Ia mencetak per halaman, di 900×600 dan 1240×860, terang dan gelap: apakah
halaman berkolom bergulir (harus 0), posisi kartu pratinjau sebelum/sesudah
kolom kiri digulir ke dasar (harus sama), dan apakah kartunya muat.
**Baseline 24 Oktober 2026: 1240×860 lulus semua; 900×600 kartu pratinjau
Clips lebih 202 px dan Watermark 107 px (kolomnya bergulir sendiri).**

Potretnya juga wajib dilihat, bukan cuma angkanya: kotak kosong, teks
terpotong, dan lambang yang tidak ada di font hanya ketahuan dari gambar (`ⓘ`
U+24D8 tampil sebagai kotak kosong berbulan-bulan sebelum ketahuan).

Sebelum menyerahkan perubahan tampilan, periksa:

- tidak ada panel yang tingginya bergantung pada isi yang bisa tumbuh;
- tiap daftar punya `max-height` + `overflow-y: auto`;
- kendali yang cuma satu angka memakai `<Stepper>` (kotak angka + tombol +/−
  bertumpuk), bukan `<input type="range">`;
- tidak ada kalimat pengantar yang mengulang apa yang sudah terlihat — SELAIN
  satu judul + satu baris subjudul di `.page-header` (keputusan pemilik 9
  Oktober 2026, `DESIGN.md` §5.2).

### 2. Halaman `/watermark` (Mark) adalah ACUAN. Salin, jangan mengarang.

Keputusan pemilik 24 Oktober 2026: Mark satu-satunya halaman yang memakai SEMUA
komponen bersama sekaligus, jadi halaman lain dicocokkan ke sana.

**Navigasi kiri = SATU rail 84px** di semua lebar (rancangan 14 Oktober 2026):
ikon + label pendek (Clips, News, Write, Caption, Mark), garis, History; kaki =
Settings, titik status AI, avatar (tema & bahasa di popup akun). Pekerjaan
berjalan tampil sebagai lencana di History. Item 48px karena 900×600 hanya punya
505 px tinggi — ukur ulang bila menambah item. Tidak ada bilah ikon di atas.

**Satu `.page-header` per halaman** (`DESIGN.md` §5.2): `<h1>` judul (satu
baris), satu baris subjudul, status job, dan SATU aksi utama. Satu kendali kecil
boleh menempel di kanan judul (`titleAction`, mis. buka artikel di tab baru).
Arah rupa (warna, huruf, radius, elevasi) di `DESIGN.md`; bagian responsifnya
(§7) DIBUANG, acuan tetap 900×600 dan 1240×860.

```
.screen
├── .page-header            <h1> + subjudul + status job + aksi utama
├── halaman BERPRATINJAU (Clips, Watermark, News → Susun kartu)
│   └── .setup-grid
│       ├── .setup-main     KIRI: yang diisi — kartu sumber/berkas, lalu kartu setelan
│       └── <Stage>         KANAN: pratinjau (stage.tsx), lebar tetap
│            ├── <StageBar>    alat di atas bingkai (kisi, garis tengah, rasio, pager)
│            ├── <StageFrame>  slot TETAP setinggi bingkai 9:16
│            ├── tools        tombol di bawah bingkai
│            └── help         satu baris bantuan, tingginya dicadangkan
└── halaman TANPA pratinjau (Captions, Writer)
    └── .io-grid
        ├── .io-in          KIRI: yang diisi (400 px)
        └── .io-out         KANAN: hasilnya (melebar)
```

Pembagian kolom (keputusan pemilik 24 Oktober 2026): **kiri = yang diisi
(sumber + setelan); kanan = yang dilihat (pratinjau atau hasil); tombol jalan di
`.page-header`.**

**Pratinjau TIDAK BOLEH bergerak atau bergetar.** Caranya BUKAN `sticky`:
halaman berkolom TIDAK bergulir, yang bergulir hanya kolom kiri/hasil
(`scrollbar-gutter: stable`). `stage.tsx` menjaga sisanya: lebar kolom tetap
(340 px, 280 px di jendela ≤ 1100), bingkai di slot setinggi 9:16, area tombol
dan bantuan setinggi tetap. Tepi atas kartu pratinjau sejajar dengan kartu
pertama kolom kiri. Halaman yang menghitung ukuran huruf dari bingkai memakai
`useStageWidth()`. Kartu di kolom terbatas wajib `flex-shrink: 0`.

**Komponen bersama — pakai, jangan salin:**

| Kebutuhan | Komponen |
| --- | --- |
| kolom pratinjau | `stage.tsx` (`Stage`, `StageFrame`, `useStageWidth`) |
| baris alat di atas pratinjau (kisi + garis tengah + rasio, + pager News) | `stage.tsx` (`StageBar`) — Clips, Mark, News |
| jendela "Preview result" (+ pager carousel) | `stage.tsx` (`Shot`) |
| kartu setelan | `tab-card.tsx` (`TabCard`, `Group`, `Row`, `Pad`) |
| dua/tiga bagian sejajar | `tab-card.tsx` (`Split side={…} mid={…} sideOff`) — jangan menulis `.mc-split` sendiri |
| warna | `color-panel.tsx` (`ColorPanel`, `toHex`, `isDark`, `parseColor`) |
| warna bebas (roda hue, S/L, HEX/RGB/CMYK/HSL, warna terakhir) | `color-wheel.tsx`, dibuka dari `ColorPanel` |
| angka | `stepper.tsx` (`Stepper`) |
| daftar berkas, nama berkas dari path | `file-list.tsx` (`FileCard`, `useFiles`, `baseName`) |
| log job | `log-panel.tsx` (`TechNotes`) |
| siklus job latar (SSE, start, cancel) | `use-job.ts` (`useJob(api, event, setError, onDone)`) |
| POST JSON ke engine | `engine.ts` (`post(path, body, fallback?)`) — jangan menulis `fetch(eng(..), {method:"POST"…})` sendiri |
| salin ke clipboard | `copy-link.tsx` (`copyText`) — `navigator.clipboard` TIDAK ADA lewat alamat IP |
| tema kartu berita (8 warna + Auto) | `themes.tsx` (`ThemePicker`, `THEMES`) — News DAN Mark |
| perataan teks (4 ikon) | `clip-steps.tsx` (`AlignPicker`, `Align`) — News DAN Mark |
| Mark: state logo/judul + seret | `mark.tsx` (`useMark`; `llm` = judul LLM per klip) — Clips, Mark, News |
| Mark: panel Logo / Headline / Template, sasaran warna judul | `mark.tsx` (`MarkLogoGroup`, `MarkHeadline`, `MarkTemplates`, `useTemplates`, `watermarkColors`) |
| Mark: lapisan di pratinjau, font pratinjau, waktu tampil | `mark.tsx` (`MarkLayers`, `MarkFont({names})`, `MarkTiming`) — engine: `watermark.ApplyLayers` |
| contoh caption (mode, gerak, bayangan) | `caption-presets.tsx` (`CaptionWords`, `captionShadow`) — kotak preset DAN pratinjau klip |

Nama warna (`white`, `yellow`, `black`, `red`) diterjemahkan HANYA oleh `toHex`,
dengan angka yang sama persis dengan `subtitle.assColor` (dulu tiga peta berbeda
membuat pratinjau tidak sama dengan hasil render).

Label = NAMA (satu-dua kata); penjelasan pindah ke `title` atau teks pilihannya
(`notes/29`).

### Resep kartu setelan (berlaku di SEMUA halaman)

Komponennya `gui/app/tab-card.tsx`; contoh hidupnya tab Text di Klip dan tab
Headline di Mark.

1. **Satu kartu setelan per halaman.** Lebih dari ~6 baris → `TabCard`, 3–5 tab
   bernama satu kata. Sedikit → `.card` + `Group` tanpa tab (Captions).
2. **Tab → `Group` (judul kecil kapital) → `Row`.** Label di kolom kiri, kendali
   di kolom kanan; semua kendali mulai di garis tegak yang SAMA.
3. **Jenis kendali tetap:** pilihan 2–4 → `Segmented`; satu angka → `Stepper`
   (pilihan cepat + angka bebas = `Segmented` + `Stepper` berdampingan,
   `.mc-combo`); nyala/mati → `Toggle`; daftar panjang (≥ 5) → `Select`; teks →
   `input`. Radio TIDAK dipakai.
4. **Warna = `<ColorPanel>`** di panel kanan: SATU palet dengan sasaran (Text /
   Highlight, Text / Box / Outline), palet bulat bersama, warna bebas, kolom
   kode. Tidak ada baris palet per warna, tidak ada `<Select>` warna.
5. **Sejajar, bukan atas-bawah** (keputusan pemilik 24 Oktober 2026). Dua grup
   dalam satu tab = SATU `Group` berisi `Split`: kiri = daftar/isian utama,
   kanan = panel rincian (label di atas kendali). Kolom tengah opsional (`mid`)
   berdiri sendiri di jendela ≥ 1400 px dan turun ke bawah kolom kiri di jendela
   lebih sempit. Panel kanan SELALU ada — dimatikan, tidak disembunyikan.
6. **Tidak ada yang muncul atau hilang** (keputusan pemilik 24 Oktober 2026).
   Baris/isian/pilihan yang belum berlaku TETAP tampil tapi dimatikan (`<Row
   off>`, `row(label, kendali, note, off)` di Klip, `<Split sideOff>`,
   `ColorTarget.off`, `<fieldset disabled>`). Contoh: warna sorot mati bila gaya
   subtitle tidak memakainya. Diukur: tinggi kartu setelan Mark sama persis saat
   kotak judul / "Set time" dinyalakan.
7. **Daftar = `.item-list`** (logo, template, …): baris padat, tombol `tiny`,
   bergulir setelah ±5 baris. Deretan kotak contoh yang banyak (preset caption)
   = satu baris yang digulir ke samping dengan panah.
8. **Penjelasan** = `note` di bawah kendali atau `desc` Toggle, satu kalimat.
9. **Berkas/sumber** = kartu sendiri DI ATAS kartu setelan.

## Pakai ulang dulu, baru menulis fungsi baru (aturan pemilik 25 Oktober 2026)

**Dilarang membuat fungsi, komponen, hook, atau konstanta baru bila yang sudah
ada masih bisa dipakai.** Sebelum menulis apa pun, cari dulu (`grep`) di tabel
"Komponen bersama" di atas untuk GUI; di engine `textx` (ExtractJSON, Snip),
`httpx` (Retry, GetImage), `config` (NewsBox*, CardPhoto*, FrameSize), `news`
(domain, clean, firstNonEmpty), `api/files.go` (within, hostPath),
`api/newscard.go` (newsPhoto, renderImage, writeFileAtomic), `api/source.go`
(thumbFile). Kalau yang ada hampir cocok, PERLUAS (parameter opsional, varian) —
jangan menyalin dengan sedikit perubahan. Salinan yang ditemukan saat bekerja
disatukan, bukan ditambah. Peninjauan 25 Oktober 2026 menemukan empat pengunduh
gambar, empat pengambil JSON LLM, dan lima pemotong teks yang sudah saling
berbeda perilaku — itulah yang dicegah aturan ini.

## Konvensi

- **Komentar** kode dalam **bahasa Indonesia**.
- **Semua yang lain dalam bahasa Inggris**: nama folder, berkas, paket, fungsi,
  variabel, konstanta, nama test, field JSON API, event SSE, nilai flag CLI,
  pesan commit, dan seluruh pesan yang dilihat pengguna.
- **Jangan `window.confirm/alert/prompt`** — konfirmasi lewat `useConfirm()`
  (`gui/app/confirm.tsx`).
- Teks antarmuka lewat kamus `gui/app/i18n.tsx` (EN sumber kebenaran; tiap kunci
  wajib punya pasangan Indonesia, dijaga TypeScript).
- Teks yang engine tulis ke kartu mengikuti parameter `lang` dari klien; default
  `en`.
- Biner eksternal, model, font (`*.ttf`), `data/`, `.env`, dan `.ref/` TIDAK
  di-commit (`.gitignore`). Teks lisensi font (`*-OFL.txt`) ikut di-commit.
- Model whisper default engine: `small` (produksi). Uji cepat: `base`.
- Video besar: audio di-stream via ffmpeg (tidak dimuat ke RAM).
- Ganti teks warna merah untuk task yang belum dikerjakan; ganti teks warna
  kuning untuk task yang perlu keputusan pemilik proyek.

## Cara mengarahkan pemilik proyek (mode belajar)

Pemilik proyek mengerjakan sendiri perubahan kodenya dan memakai agen sebagai
pengarah, bukan pengetik. Kalau ia minta "arahkan", "aku mau belajar", atau
menyebut "jangan rubah code": **jangan menyunting berkas apa pun** — termasuk
setelah beberapa giliran berlalu; larangan itu berlaku sampai ia mencabutnya.

Bentuk arahan — **satu bagian per suntingan**, urut dari baris paling bawah ke
paling atas supaya nomor baris tidak bergeser saat ada baris yang dihapus:

1. nomor baris
2. blok kode **Dari:**
3. blok kode **Jadi:** (atau "hapus seluruh barisnya")
4. **Kenapa:** 1–3 kalimat — alasannya, bukan sekadar apa yang berubah

Tutup dengan perintah verifikasi (build/vet/test/grep) dan angka yang harus
muncul kalau benar. Sebutkan angka harapan itu **sebelum** perintahnya
dijalankan; itu yang membedakan "kelihatan jalan" dari "terbukti jalan".

Yang bikin arahan gagal:

- **penjelasan konsep panjang dulu, daftar suntingan belakangan.** Sertakan
  alasan di dalam tiap suntingan.
- **memberi banyak langkah sekaligus saat ia bilang bingung.** Turunkan jadi
  satu suntingan, minta hasil build, baru lanjut.
- **kalimat yang bisa ditafsir lebih luas dari maksudnya.** "`ctx` hilang dari
  pemanggilan" terbaca sebagai "buang `ctx` dari tanda tangan fungsi" dan
  memecahkan tujuh tempat lain. Sebut ruang lingkupnya dengan tegas.

Kalau ia melaporkan error, **baca keadaan berkasnya lebih dulu** (build, diff,
baris yang disebut) sebelum menebak sebabnya.
