# Serah terima — 6 Agustus 2026 (malam)

Ditulis sebelum mesin dimatikan. **Baca ini dulu**, lalu `CLAUDE.md`. Rinciannya
di `notes/32`–`35`; ini peta dan daftar kerjanya.

Keadaan repo saat catatan ini ditulis: pohon kerja **bersih**, rilis terakhir
**v0.6.1**, `tauri.conf.json` juga 0.6.1.

> **Ditulis ulang 6 Agustus malam**, sesudah kerja server LLM. Butir 2 selesai,
> butir 1 sebagian; pohon kerja TIDAK lagi bersih — ada perubahan di
> `internal/api`, `internal/score/ollama`, `internal/setup`, dan `gui/app`.
> Terverifikasi: `go vet` bersih, `go test ./...` lulus, `tsc` bersih, dan ukur
> UI **klip 0/0 · kartu 0/0 di 1240×860**.

## Yang sudah dikerjakan (dan angkanya)

**Tampilan tidak bergerak** (`notes/32`)

- Lingkaran umpan balik ukuran pratinjau diputus. Bingkai **262×465 pada subSize
  72 DAN 140** — angka yang sama persis.
- `.screen-head` dibuang: galat & peringatan jadi lapisan melayang
  (`alerts.tsx`), kemajuan job pindah ke panel Start yang tempatnya permanen.
- Peringatan di dalam panel jadi lambang di label (`warn.tsx`); di sel tanpa
  label ia melayang. Terukur: tombol **top 91 sebelum & sesudah**, panel
  **420 px** di kedua keadaan.
- Semua kendali sebaris memakai satu tinggi (`--ctl-h: 32px`).
- Jendela dikunci **1240×860** (satu-satunya ukuran yang 0/0 di semua halaman).

**Riwayat & keluaran**

- Riwayat job bertahan: satu JSON per job di `<DataDir>/jobs/`, klip yang
  berkasnya hilang dibuang saat dibaca.
- Kartu jadi kisi yang mengalir ke bawah; klip tetap deret mendatar per job.
- Pilih dengan klik / Ctrl+klik / Shift+klik; **unduh massal** satu zip lewat
  `GET /api/download`.
- Halaman Results dibuang (isinya bagian dari Output history).

**LLM lokal** (`notes/33`, `notes/35`)

- Alamatnya DICARI sendiri: `OLLAMA_HOST` → localhost → gerbang WSL →
  `host.docker.internal`, plus port server lain. Alamat publik ditolak.
- Engine tahu sistem asalnya: `Windows` · `WSL` · `Linux` · `macOS` · `remote`,
  tampil di label dan di tiap baris daftar model.
- Bukan Ollama saja: satu klien untuk semua server bergaya OpenAI (llama.cpp,
  LocalAI, llamafile, vLLM, Aphrodite, LiteLLM, Exo).
- Model dinilai dari metadata (parameter, konteks, kemampuan, lokal vs cloud);
  yang kecil DITANDAI, bukan ditolak. Model `*-cloud` ditolak.
- Koreksi transkrip menyesuaikan diri: potongan yang balasannya tak terbaca
  **dipecah dua** sampai model sanggup; `num_ctx` mengikuti konteks model.

**Pesan & komponen** (`notes/34`)

- Tidak ada lagi `./setup.sh` di pesan pengguna — semuanya menunjuk halaman
  Requirements. Nama baris = nama berkas sebenarnya (`whisper-cli.exe`).
- `applyPaths()` dipanggil setelah pemasangan DAN tiap `/api/requirements`
  ditanya: tidak perlu restart, dan "Check again" benar-benar memeriksa ulang.
- Requirements MENJALANKAN `ffmpeg -version`/`ffprobe -version`.
- Notifikasi "ada yang kurang" membawa tombol **Install N missing**.
- Video tanpa trek suara dikenali sebelum ekstraksi + lencana **⚠ no sound** di
  pratinjau. (Ini yang dulu muncul sebagai "Invalid argument" pada audio.wav.)

## Yang berikutnya, berurut menurut manfaat

Daftar ini disaring ulang **6 Agustus 2026** dan tiap butirnya diperiksa lagi ke
kode — yang tertulis di bawah adalah keadaan sesudah pemeriksaan itu, bukan
keadaan saat catatan ini pertama ditulis.

### 1. Uji server LLM sungguhan — SEBAGIAN (6 Agustus 2026 malam)

Tidak lagi server tiruan saja. Diuji dengan **Qwen2.5 3B Q4_K_M**, semua biner
ada di `~/llm-test/`:

| Server        | Diuji? | Hasil                                                     |
| ------------- | ------ | --------------------------------------------------------- |
| Ollama        | dipakai sejak awal | `format` benar-benar memaksa bentuk JSON      |
| **llama.cpp** | **YA** (b10295) | `/api/tags` 404 → `KindOpenAI`; `json_schema` + `strict` DIHORMATI; `meta` memberi `n_ctx`/`n_params`/`size`/`ftype` |
| **KoboldCpp** | **YA** | Jalan, TAPI dua kejutan — lihat di bawah                    |
| **Jan** | **terpasang & jalan** (7 Agu) | AppImage jalan lewat WSLg. Server API-nya masih perlu dinyalakan di jendelanya: Settings → Local API Server |
| GPT4All | terhalang | Pemasangnya butuh `libxkbcommon-x11.so.0`: `sudo apt install -y libxkbcommon-x11-0 libxcb-cursor0` |
| LM Studio | belum terunduh | URL pemasangnya di balik JS, tidak bisa diambil dari terminal |
| llamafile     | gagal jalan | APE terhalang WSLInterop di WSL (butuh `sudo`). Tidak masuk daftar |

**Dua kejutan dari KoboldCpp**, keduanya sudah ditangani:

1. Ia **MENIRU `/api/tags` milik Ollama** — lengkap dengan kunci `models`. Jadi
   `kindOf` melabelinya `KindOllama` dan engine memakainya lewat `/api/chat`
   (yang ternyata berhasil). Tapi `serverName` ikut menyebutnya "Ollama", jadi
   baris yang menyala di Requirements adalah aplikasi yang bahkan tidak
   terpasang. **Perbaikannya: `serverName` memeriksa PORT dulu, jenis protokol
   belakangan.** Dijaga `TestKoboldCppIsNotCalledOllama`.
2. Ia **mengabaikan `format`**. Diminta `{"picks":[…]}` ia menjawab
   `{"moment2":45,"moment5":60}`. Bentuk balasannya bergantung pada model
   menuruti prompt, bukan pada pagar server. Ditulis apa adanya di barisnya.

Kalau menguji sisanya nanti: nyalakan servernya, lalu bandingkan `/api/tags`
(200 = akan dianggap Ollama) dan balasan `response_format: json_schema`. Kalau
bentuknya tidak dipaksa, **tulis itu di `Detail` barisnya** — jangan diam-diam
dimasukkan sebagai "didukung".

> Catatan cara kerja, ditulis sesudah ditegur: klaim "butuh jendela jadi tidak
> bisa diuji dari terminal" itu **salah** — `DISPLAY=:0` ada di mesin ini
> (WSLg), dan Jan memang jalan begitu dicoba. Jangan mengecilkan lingkup
> permintaan berdasarkan hambatan yang belum diuji; coba dulu, baru laporkan
> apa yang benar-benar menghalangi.

### 2. Server LLM di UI — SELESAI

Bukan cuma kotak alamat: seluruh daftar server kini ADA di aplikasi.

**Halaman Requirements → "Separate applications"** menampilkan enam baris dari
`setup.LLMServers` (`engine/internal/setup/setup.go`): Ollama, LM Studio, Jan,
llama.cpp, KoboldCpp, GPT4All. Tiap baris membawa port bawaannya, cara
menyalakan servernya, tautan unduh, dan **status ujinya sendiri** ("Tested:
keeps JSON replies in shape" / "Not tested with Clipper yet"). Yang sedang
berjalan ditandai hijau dengan alamat + daftar modelnya. Tidak satu pun
`Required`, jadi titik merah "ada yang kurang" tidak menyala karenanya.

**Panel Settings (gerigi di rail)** memuat kelompok **Server**: nama server yang
menjawab, **daftar SEMUA server yang menjawab** (tinggal diklik), kotak alamat +
Save untuk port tak dikenal, dan kotak kunci. **Di sana, bukan di halaman klip** —
diminta begitu, dan memang benar: keduanya disetel sekali lalu dilupakan,
sedangkan menaruhnya di halaman klip memaksa kisi mesin skor melipat ke baris
kedua.

Daftar itu datang dari **`DiscoverAll`** (`discover.go`), yang mengembalikan
SEMUA kandidat yang menjawab, bukan cuma pemenangnya. Ongkosnya nol: semua
kandidat memang sudah diketuk berbarengan, yang berubah cuma berapa hasil yang
disimpan. Alamat yang menunjuk server sama (localhost vs 127.0.0.1) dilipat jadi
satu lewat `samePort`. Terbukti di mesin ini dengan **tiga server hidup
sekaligus** — Ollama 11434, llama.cpp 8080, KoboldCpp 5001 — dan mengklik
llama.cpp benar-benar memindahkan engine ke sana (`llm_url` jadi
`http://127.0.0.1:8080`, label berubah jadi `✓ llama.cpp`).

Dua jebakan yang ditemukan saat mengerjakannya:

- **`<Select>` TIDAK BISA dipakai di dalam `<Popover>`** — popup di dalam popup,
  yang di dalam tertutup begitu dibuka; diuji dengan klik tetikus sungguhan
  lewat CDP, `.sel-list` tidak pernah masuk DOM. Daftar servernya karena itu
  memakai baris `<button>` biasa (`.srv-row` di `globals.css`), bentuk yang sudah
  dipakai daftar komponen di panel yang sama.
- Memilih server **tidak boleh mengirim `llm_api_key: ""`** — field pointer di
  `postSettings` menafsirkannya sebagai "hapus kunci tersimpan". Kunci hanya
  ikut terkirim bila memang diketik.

**Deteksi ≠ terpasang.** Yang muncul di daftar adalah server yang SEDANG
MENJAWAB di sebuah port. Berkas pemasang yang tergeletak di disk tidak dihitung,
dan itu benar — sempat jadi pertanyaan ("kita sudah unduh banyak LLM, kenapa
belum terdeteksi?"). Jawabannya: unduh ≠ pasang ≠ jalan.

### Unduhan model Ollama: aturan notes/25 pernah dilanggar di sini juga

Ditemukan 7 Agustus 2026 dari pertanyaan "aku baru menekan tombol unduh gemma —
dari mana aku tahu ia sedang diproses?". Jawabannya saat itu: **tidak bisa
tahu**, dan ternyata unduhannya memang sudah mati. Buktinya terukur: berkas
parsial **5,44 GB dari 5,44 GB** tergeletak **14 menit tanpa bertambah**, dan
gemma2 tidak pernah muncul di `/api/tags`.

Dua sebabnya, keduanya sudah diperbaiki:

- `ollamaPull` memanggil `ollama.Pull(r.Context(), …)` — unduhan hidup di dalam
  permintaan halaman, persis yang `notes/25` larang. Sekarang ia dijalankan di
  latar dengan `context.Background()` lewat **mesin `installs` yang sudah ada**:
  `POST /api/ollama/pull` menjawab **202 seketika**, kemajuannya disiarkan di
  SSE yang sama dengan pemasangan komponen (`/api/requirements/events`), dengan
  id `llm-model:<nama>`.
- `ollama.Pull` mengirim `"stream": false` — satu permintaan HTTP yang diam
  belasan menit tanpa satu pun angka. Sekarang `stream: true` dan tiap baris
  NDJSON-nya (`status`, `completed`, `total`) diteruskan sebagai `PullProgress`.

Di GUI, halaman **BERLANGGANAN saat dibuka**, bukan hanya sesudah tombolnya
ditekan: memuat ulang halaman di tengah unduhan 5 GB tetap menampilkan
kemajuannya, sebab engine mengirim cuplikan keadaan saat SSE tersambung.
Terbukti — unduhan dimulai lewat `curl`, dan halaman yang dibuka sesudahnya
tetap melaporkannya sampai `✓ Model gemma2 is ready`.

Kemajuannya tampil **di label**, bukan cuma di tombol: tombol unduhnya tinggal
di dalam popup peringatan, jadi selama unduhan berjalan yang terlihat hanya
lambang ⚠. Dan ia **menggantikan** nama server, tidak ditambahkan di sebelahnya —
"Model · Ollama · 51%" terpotong jadi "Model · Ollama …" (diukur, bukan
dikira).

Yang berubah di engine:

- Alamat disimpan sebagai **`OLLAMA_HOST`** di `.env`, bukan variabel baru —
  `Candidates()` sudah membacanya lebih dulu dan memenangkannya atas semua
  tebakan, jadi CLI ikut memakainya tanpa jalur tambahan.
- Kunci dari **`LLM_API_KEY`** menggantikan `Bearer local` yang dipaku di DUA
  tempat: `openai.go` (permintaan) dan `discover.go` (**pemeriksaan**). Yang
  kedua yang fatal — server ber-auth membalas 401, `probeJSON` mengembalikan
  `false`, dan ia dilaporkan "tidak jalan", bukan "salah kunci".
- Semua field `postSettings` jadi **pointer**: menyimpan satu tidak boleh
  menghapus dua lainnya.
- `ollama.ResetCache()` dipanggil setelah alamat/kunci berubah — tanpa itu
  alamat lama masih dipakai sampai 30 detik berikutnya.
- `openAIModels` membaca `data[].meta` → `Context`/`Bytes`/`Params`/`Quant`.
  `Context` yang paling penting: ia jadi `Client.NumCtx`, dan untuk server
  OpenAI nilainya SELALU 0 sebelum ini.
- Port ditambah: **1337** (Jan), **5001** (KoboldCpp), **4891** (GPT4All).

Dua hal yang saat itu disebut "keputusan pemilik" sudah diputuskan sambil
jalan: alamat disimpan di `.env` mengikuti pola kunci Claude, dan alamat yang
DIKETIK pengguna tidak melewati saringan IP privat `hostURL()` — saringan itu
tetap berlaku untuk tebakan otomatis saja.

### 3. Kartu berita: gambar & paragraf SUDAH

Dilaporkan 7 Agustus 2026 dari tab News cards: kartu jadi tanpa gambar.
**Empat perbaikan**, semuanya terukur — lihat (a)–(d) di bawah.

Reproduksi (artikel Blogspot, `tabloidlugas.com`):

```bash
curl -s -X POST http://127.0.0.1:8787/api/news/article \
  -H 'content-type: application/json' \
  -d '{"url":"https://www.tabloidlugas.com/2026/08/opini-di-balik-temuan-995-senjata-sekolah-konflik-yayasan-dan-pencarian-kebenaran.html"}'
# title: benar · image: "" · paragraphs: 0
```

**SUDAH DIPERBAIKI — DUA hal, dan yang kedua yang sebenarnya dikeluhkan.**

**(a) Klik baris daftar tidak pernah mengambil artikelnya.**
`news/page.tsx` memasang `onClick={() => useArticle(a)}` — objek `a` itu dari
RSS, dan **RSS tidak membawa gambar**. Jadi kartu selalu jadi tanpa foto.
Satu-satunya cara yang bekerja adalah jalur memutar: tekan `copy` (yang
diam-diam meresolusi tautannya), tempel ke kotak Fetch, baru gambarnya muncul —
persis yang dilaporkan. Sekarang klik memanggil `openItem`, yang MENGAMBIL
artikelnya lewat `/api/news/article`. Ringkasan RSS tetap dipasang lebih dulu
supaya kliknya terasa langsung menjawab, lalu ditimpa artikel lengkapnya.

**(b) Langkah resolve yang hilang di engine.**

`api.newsArticle` meneruskan URL apa adanya ke `news.FetchArticle`. Tautan dari
daftar berita menunjuk `news.google.com/rss/articles/CBMi…`, jadi yang terbaca
adalah halaman Google sendiri. Terukur, artikel yang sama:

| | sebelum | sesudah |
| --- | --- | --- |
| title | `Google Berita` | judul artikel asli |
| image | logo Google | `bacaini.id/wp-content/uploads/…` |

Langkah `news.Resolve` sebenarnya SUDAH ADA — tapi hanya di tombol salin-tautan
di GUI (`copyLink`). **Satu langkah yang hidup di satu tombol saja adalah
langkah yang pasti terlewat di tombol berikutnya** — tempatnya di engine, dan
sekarang di sana. Itu juga yang membuat (a) mungkin: klik daftar boleh mengirim
tautan Google mentah, engine yang membukanya. Murah untuk tautan biasa:
`Resolve` mengembalikannya apa adanya, dijaga
`TestResolvePassesOrdinaryLinksThrough`.

Terbukti dari ujung ke ujung dengan tautan yang PERSIS dikirim tombol klik:

```
dikirim   : https://news.google.com/rss/articles/CBMiqgFBVV95cUxNVH…
url balik : https://centralnews.id/polisi-selidiki-temuan-995-senjata-api-…
image     : https://centralnews.id/wp-content/uploads/2026/08/Screenshot_…
```

**(c) Gambar dicari BERLAPIS** — `og:image` → JSON-LD `NewsArticle.image` →
`<img>` pertama DI DALAM badan artikel. Lapisan ketiga sengaja dibatasi ke badan
artikel: di tingkat halaman, gambar pertama hampir selalu logo situs atau iklan,
dan kartu berlogo situs lebih buruk daripada kartu polos. Dijaga
`TestArticleImageFallbacks`.

**(d) Paragraf dari markup Blogspot** — `reInlineBlock` menangkap blok yang
isinya teks + tag SEBARIS (`<a>`, `<b>`, `<span>`), bukan hanya teks polos.
Karena isinya dibatasi begitu, blok yang memuat `<div>` lain tidak cocok —
hasilnya selalu blok terdalam, bukan pembungkus sehalaman. Ditambah
`articleBodyHTML` yang mempersempit ke badan artikel lebih dulu, memakai penanda
yang SAMA dengan pencarian gambar (satu daftar, dua keperluan). Terukur di
halaman tabloidlugas: **0 → 17 paragraf**. Dijaga
`TestParagraphsInsideDivsWithInlineTags` (termasuk: menu & kaki artikel tidak
boleh ikut).

**Sapuan 10 hasil pencarian** (7 Agu 2026), untuk menjawab "yang lain gimana?":

| | |
| --- | --- |
| punya gambar | **7** (centralnews, viva ×2, bacaini, kompas.tv, achmadnurhidayat, publika) |
| tidak berfoto | 1 — tabloidlugas.com, memang tidak punya |
| bukan artikel | 1 — `kompas.tv/tag/995-senjata-api`, halaman TAG; Google News memang kadang menautkan halaman daftar |
| gagal resolve | 1 — "the link has not reached the original article yet", coba lagi |

Yang MASIH tersisa:

1. **Halaman TAG/daftar tidak ditolak lebih awal.** `kompas.tv/tag/…` diproses
   seperti artikel biasa lalu menghasilkan kartu tanpa isi. Lebih baik dikenali
   dan dikatakan.
2. **(sudah selesai — lihat (d) di atas)** Paragraf dari markup Blogspot. `content.go:117-118` mencari
   `<p>…</p>` (halaman itu cuma punya 2) atau `<div>` yang isinya teks polos
   ≥60 karakter (`[^<]{60,}`) — dan badan tulisan Blogspot penuh tag inline di
   dalamnya, jadi pola itu tidak pernah cocok. Akibatnya tombol Analyse &
   Paragraphs tidak punya bahan apa pun.

Yang harus diputuskan sebelum mengerjakan: gambar cadangan diambil dari `<img>`
pertama **di dalam badan artikel** (butuh menemukan badan itu lebih dulu) atau
dari `<img>` terbesar di halaman. Yang pertama lebih benar, yang kedua lebih
mudah dan gampang salah mengambil logo situs — halaman itu punya dua logo
"LUGAS 28th" sebagai dua `<img>` pertamanya.

### 3b. Tautan hasil pencarian yang TIDAK BISA diresolusi — BELUM SELESAI

Ditemukan 7 Agustus 2026 saat menelusuri keluhan "harus copy dulu baru gambarnya
muncul". Yang sudah dibetulkan ada di butir 3; yang ini tersisa.

Sebagian tautan `news.google.com/rss/articles/CBMi…` **tidak pernah berpindah**
ke alamat medianya di Chrome headless. Bukan soal terlalu cepat — anggaran waktu
virtualnya sudah 15 detik (`DumpDOM(ctx, url, 15000)`), dan tiga percobaan
berturut-turut pun gagal dengan hasil yang sama.

Dua jalan pintas sudah DIUJI dan tidak bisa dipakai:

- **`curl -L`** tidak menolong: Google menjawab 200 dengan halaman JavaScript,
  tidak ada pengalihan HTTP sama sekali.
- **Membongkar blob di dalam tautannya** tidak menolong: base64 `CBMi…` tidak
  memuat URL apa pun dalam bentuk polos.

Sisa jalan yang masuk akal, belum dikerjakan:

1. Meminta DOM SESUDAH perpindahan, bukan sesudah anggaran waktu habis — mis.
   memantau `document.location` lewat CDP alih-alih `--dump-dom` sekali tembak.
2. Menerima kegagalannya dan mengatakannya terus terang: baris yang tidak bisa
   dibuka ditandai di daftar, bukan gagal diam-diam saat diklik.

Yang PENTING dicatat supaya tidak salah diagnosis lagi: saat tautan itu gagal,
**tombol copy juga gagal** — keduanya memakai `news.Resolve` yang sama. Jadi
"copy berhasil, klik tidak" sudah tidak berlaku sejak butir 3 diperbaiki.

### 4. Layar 1366×768 — BELUM

`desktop/src-tauri/tauri.conf.json` masih `minWidth: 1240`, `minHeight: 860`;
860 + bilah judul + taskbar ≈ 940, tidak muat. Kalau jadi masalah nyata, yang
dikerjakan adalah membuat halaman klip muat di jendela lebih pendek — BUKAN
menurunkan angka minimumnya; tabel ukur di `notes/33` menunjukkan akibatnya
kalau diturunkan. Bahannya sudah ada: `<Section>` (`gui/app/section.tsx`) sudah
bisa diciutkan, tinggal dipakai untuk kelompok setelan.

### 5. Deploy situs otomatis — DITUTUP pemilik

Dianggap selesai; deploy dijalankan sebagaimana adanya. Catatan faktual supaya
sesi berikutnya tidak bingung: `.github/workflows/desktop.yml` di repo ini
BERHENTI di langkah "Publish release" — tidak ada `repository_dispatch`, jadi
kalau otomatisasinya ada, ia hidup di sisi `gemgum.github.io`, bukan di sini.

### 6. Galat ffmpeg lama di Windows — SETENGAH JALAN

Akar yang bisa dibuktikan sudah ditutup: `checkRuns`
(`engine/internal/setup/setup.go:215`) BENAR-BENAR menjalankan
`ffmpeg -version` / `ffprobe -version`, dan yang gagal dilaporkan tidak
terpasang dengan pesan sistemnya apa adanya (maks 300 karakter) di `Detail`.

Dua hal tersisa:

- **Teks galatnya masih belum ada.** Belum ada yang membuka halaman Requirements
  di mesin Windows itu. Satu tangkapan layar baris ffmpeg/ffprobe sudah cukup;
  sebelum itu, apa pun yang dikerjakan hanya tebakan (`notes/31` butir 1).
- **`whisperStatus` (`setup.go:235`) BELUM ikut.** Ia `os.Stat`, ketemu,
  `return` di baris 246–249 — tidak pernah memanggil `checkRuns`. Padahal
  laporan aslinya berbunyi *"untuk whisper tadi..."*, jadi kalau yang tidak bisa
  jalan justru `whisper-cli.exe`, Requirements MASIH bilang hijau. Bedanya:
  `-version` tidak dijamin ada di whisper-cli — cek dulu flag mana yang keluar
  bersih (`-h` mungkin lebih aman).

## Yang DIBUANG dari daftar (6 Agustus 2026)

Bukan ditunda — diputuskan tidak dikerjakan. Jangan dihidupkan lagi tanpa alasan
baru:

- **Menghapus satu JOB utuh dari riwayat.** Menghapus per klip sudah cukup.
- **Sumber private + repo rilis publik.** Repo tetap seperti sekarang.

## Dua jebakan yang memakan waktu di sesi 6 Agustus

Keduanya membuat saya membaca hasil yang SALAH selama beberapa putaran. Kalau
angka dan uji unit bertentangan, curigai keduanya lebih dulu.

**1. `clipper serve` yang gagal bind DIAM SAJA di latar.** Port 8787 masih
dipegang proses lama, `serve` baru menulis "address already in use" ke lognya
lalu mati — dan yang menjawab semua `curl` berikutnya adalah **biner sebelum
perbaikan**. Terjadi DUA KALI. Gejalanya khas: uji unit bilang "KoboldCpp",
API bilang "Ollama". Sebelum percaya jawaban engine, pastikan yang mendengarkan
itu proses yang baru:

```bash
ss -lptn 'sport = :8787'          # ada yang pegang? bunuh dulu
head -2 <log serve>               # baris pertama harus banner, bukan galat bind
```

**2. `pkill -f <pola>` ikut membunuh shell yang menjalankannya**, sebab pola itu
juga ada di baris perintahnya sendiri (exit 144). Pakai `pgrep -af` untuk
melihat PID-nya lebih dulu, lalu `kill <pid>`.

## Cara memeriksa sebelum menyerahkan apa pun

```bash
./bin/clipper serve                       # terminal lain
~/.cache/ms-playwright/chromium-*/chrome-linux64/chrome \
  --headless --remote-debugging-port=9333 --user-data-dir=/tmp/cdp about:blank &
node scripts/measure-ui.mjs http://127.0.0.1:8787 /tmp/shots
# SIZES='[["1240x860",1240,765]]' untuk ukuran lain
```

**Lihat potretnya, bukan cuma angkanya.** Lalu: `go vet ./...` ·
`go test ./...` · `npx tsc --noEmit --noUnusedLocals --noUnusedParameters` ·
`npm run build`.

---

## Tambahan 7 Agustus 2026 (sore): butir 3b SELESAI + versi di panel setelan

### Pengalih Google News dibuka TANPA browser

Butir 3b di atas ("tautan hasil pencarian yang tidak bisa diresolusi") ditutup,
dan bukan dengan menunggu lebih lama. Jalan ketiga yang dulu tidak terpikir:
**panggil RPC yang dipakai halaman Google itu sendiri saat hendak berpindah.**

`engine/internal/news/google.go`. Dua permintaan HTTP biasa:

1. `GET` halaman pengalihnya → ambil `data-n-a-ts` (stempel waktu) dan
   `data-n-a-sg` (tanda tangan) yang tertanam di HTML-nya.
2. `POST news.google.com/_/DotsSplashUi/data/batchexecute` dengan rpcid
   **`Fbv4je`**, membawa id artikel + kedua nilai itu. Balasannya alamat asli.

Balasan langkah 2 bukan JSON utuh — baris penjaga `)]}'`, lalu array yang salah
satu elemennya JSON BERBENTUK TEKS. Diurai dua tahap di `parseGarturlres`,
dijaga `TestParseGarturlres` (tanpa jaringan).

**Terukur, 10 hasil pencarian yang sama seperti sapuan sebelumnya:**

| | browser (sebelum) | RPC (sesudah) |
| --- | --- | --- |
| berhasil | 9/10 | **10/10** |
| waktu per tautan | 3–15 dtk | **112–176 md** |

Ujung ke ujung lewat GUI (CDP, klik sungguhan): cari → klik baris pertama →
kartu langsung berisi foto, judul, teks, sumber. `POST /api/news/article` untuk
4 hasil teratas: **4/4 berdapat gambar, 0,1–0,6 detik**.

Browser TIDAK dibuang: ia tetap jalan cadangan di `Resolve` dan `FetchContent`
bila bentuk halaman Google berubah. Yang berubah cuma urutannya — RPC dulu,
browser belakangan.

Uji hidupnya ada, dan sengaja dilewati tanpa jaringan:

```bash
CLIPPER_TEST_NET=1 go test ./internal/news/ -run TestDecodeGoogleNewsManual -v
```

### Yang MASIH tidak berfoto: baris daftarnya sendiri

RSS Google News tidak membawa gambar sama sekali, jadi 24 baris hasil pencarian
tetap tanpa thumbnail — yang berfoto adalah kartunya, sesudah diklik. Memberi
thumbnail pada daftar berarti membuka 24 tautan ke Google tiap kali orang
mencari (±7 detik dan 24 permintaan). **Keputusan pemilik proyek**, bukan
sesuatu yang dikerjakan diam-diam.

### Versi engine tampil di panel setelan

`/api/health` kini menjawab `{"status":"ok","version":"…"}`; nilainya dari
`main.version`. GUI menariknya saat panel dibuka.

Angkanya datang dari **`desktop/src-tauri/tauri.conf.json`**, satu tempat untuk
seluruh aplikasi: `build.sh` dan `build-windows.sh` membacanya lalu menyuntiknya
lewat `-ldflags "-X main.version=…"`. Sebelum ini build lokal memakai bawaan
`main.version` dan panel melaporkan **0.1.0-dev** — angka yang tidak pernah
dirilis siapa pun. Sekarang `./bin/clipper version` dan `/api/health` sama-sama
menjawab **0.7.0**. Di CI tidak berubah: workflow rilis memakai nomor tag, dan
ia SUDAH menolak tag yang tidak cocok dengan tauri.conf.json.

Tempatnya **di baris judul "Settings"**, rata kanan — bukan kelompok "About"
sendiri. Alasannya terukur: sebagai baris kaki di sebelah tautan Requirements,
keduanya melipat jadi dua baris dan panel naik 427 px; di baris judul ia nol
tambahan dan panel tetap **301 px, scroll 0**. Ukur UI tetap **klip 0/0 ·
kartu 0/0 di 1240×860**.

### "the feed parsed but contains no articles" pada pencarian — pesan yang salah alamat

Dilaporkan langsung sesudah perbaikan di atas. Bukan kegagalan jaringan:
**pencarian yang nol hasil** memakai pesan milik feed media, jadi orang yang
baru saja mengetik kata kunci disuruh "check the feed URL" — alamat yang bahkan
tidak pernah ia ketik. Reproduksi: `?q=asdkjhqwe`.

`parseFeed` sekarang mengembalikan `errNoArticles` (sentinel di `rss.go`), dan
`Search` menerjemahkannya jadi `no news found for "…" — try fewer or more
common words`. Feed media tetap memakai kalimat lama, sebab di sana kosong
memang berarti alamatnya keliru. Dijaga `TestEmptyFeedIsItsOwnError`.

---

## Tambahan 7 Agustus 2026 (malam): uninstaller & panel setelan

### Uninstaller meninggalkan data pengguna

Dilaporkan: uninstall tidak membuang semuanya. Benar, dan sebabnya bisa
disebut: uninstaller Tauri hanya tahu folder programnya, sedangkan SEMUA data
Clipper ada di `%LOCALAPPDATA%\Clipper` (`config.userDataDir`) — model whisper
(sampai 3 GB), whisper.cpp & ffmpeg hasil unduhan halaman Requirements, cache
transkrip, riwayat job, `.env`, plus klip & kartu yang tersimpan di folder
bawaan. Tidak satu pun ikut terhapus.

Perbaikannya `desktop/src-tauri/installer-hooks.nsh` + `installerHooks` di
`tauri.windows.conf.json`: hook `NSIS_HOOK_POSTUNINSTALL` MENAWARKAN membuang
folder itu.

Tiga keputusan di dalamnya, semuanya sengaja:

- **Ditawarkan, bukan dipaksa.** Memasang ulang versi baru jauh lebih enak
  kalau model 3 GB-nya tidak perlu diunduh lagi.
- **Tombol bawaannya "No"** (`MB_DEFBUTTON2`). Di antara menyisakan berkas dan
  menghapus klip orang, yang kedua jauh lebih mahal.
- **Uninstall senyap tidak menghapus apa pun** (`IfSilent`): tidak ada yang
  bisa menjawab, dan diamnya bukan berarti setuju.

Yang TIDAK tertutup, dan jangan diklaim tertutup: **.msi (WiX) tidak punya
hook ini** — butuh custom action tersendiri. Situs menautkan
`Clipper-setup.exe` (NSIS), jadi jalur yang dipakai orang sudah benar. Paket
`.deb`/AppImage juga tidak menyentuh `~/.local/share/clipper`, sesuai kebiasaan
Linux.

Diverifikasi sejauh yang bisa dari Linux: nama & letak kuncinya dicocokkan ke
`config.schema.json` milik `@tauri-apps/cli` yang benar-benar dipakai
(`BundleConfig.windows` → `WindowsConfig.nsis` → `NsisConfig.installerHooks`,
bertipe string). **Perilaku dialognya sendiri baru terbukti pada pemasang
Windows** — uji sekali saat rilis berikutnya turun.

### Panel setelan: 16 baris komponen jadi 4

Diminta begitu, dan memang benar — panel ini melaporkan keadaan, bukan
mengelola komponen. Enam model whisper yang cukup dipunya satu dan enam
aplikasi LLM yang cukup jalan satu menjawab "siap atau belum" dengan menyuruh
orang membaca dulu.

| Baris | Hijau kalau |
| --- | --- |
| Library | ffmpeg DAN ffprobe ada |
| Transcribe | whisper.cpp ada DAN setidaknya satu model ada |
| LLM | setidaknya satu server LLM terdeteksi |
| Chrome | Chrome/Edge/Chromium ketemu |

Isi tiap kelompok pindah ke `title` — yang ditanya orang begitu satu baris
merah adalah "yang mana yang kurang".

Ini juga MEMPERBAIKI titik merah di gerigi: model whisper ditandai
`required: false` satu per satu (karena cukup punya salah satu), jadi mesin
yang punya whisper tanpa satu pun model dulu tampil hijau — padahal job
pertamanya pasti berhenti. Sekarang hitungannya per KELOMPOK.

Terukur: panel 415 px → **379 px**, scroll 0; halaman tetap klip 0/0 ·
kartu 0/0 di 1240×860. Keadaan kurang diuji sungguhan dengan
`CLIPPER_MODELS_DIR` kosong: baris Transcribe merah + "missing", titik merah di
gerigi menyala, dan notifikasi lama ("Speech model … has not been downloaded")
tetap muncul.
