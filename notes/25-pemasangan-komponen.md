# Pemasangan komponen sekali klik

Selesai 4 Agustus 2026 (bagian pertama). Rencananya di `23-aplikasi-desktop.md`.

## Bentuknya

Satu halaman **Requirements** (tab ketiga di GUI): tiap komponen satu baris,
lampu hijau/merah, ukuran unduhan, dan tombol. Engine yang tahu apa yang ada dan
apa yang kurang — GUI hanya menampilkan jawabannya.

| Endpoint | Isi |
| --- | --- |
| `GET /api/requirements` | status semua komponen + daftar yang wajib tapi belum ada |
| `POST /api/requirements/install` | pasang satu komponen, alirkan progres (SSE) |
| `POST /api/requirements/remove` | hapus model yang sudah diunduh |

| `GET /api/requirements/events` | menonton kemajuan semua pemasangan (SSE) |
| `POST /api/requirements/path` | menunjuk program yang sudah ada di komputer |

## Pemasangan berjalan di LATAR (diperbaiki 4 Agustus 2026)

Versi pertama memanggil `setup.Install(r.Context(), …)` — unduhannya hidup di
dalam permintaan HTTP halaman. Akibatnya di Windows: pengguna menekan Install,
pindah jendela, WebView2 ditidurkan sistem, koneksi putus, konteks dibatalkan,
dan unduhan 111 MB berhenti. Dari sisi pengguna terlihat "mengulang dari nol".

Sekarang `POST …/install` memulai goroutine dengan `context.Background()` lalu
**langsung menjawab 202**. Kemajuannya disiarkan lewat `…/events`, dan pesan
pertama ke pelanggan baru selalu berisi keadaan terkini — jadi halaman yang baru
dibuka langsung melihat unduhan yang sedang berjalan, termasuk yang dimulai
sebelum halaman itu ada.

Pelajarannya persis yang sudah tertulis di `23-aplikasi-desktop.md`: *"Pola
SSE-nya sudah ada di `job` — pakai ulang, jangan bikin mekanisme kedua."* Yang
dipakai ulang waktu itu hanya **format** SSE-nya, bukan pola kerjanya. Job klip
sejak awal berjalan di latar dan GUI hanya berlangganan; pemasangan komponen
seharusnya begitu juga sejak awal.

Aturan yang berlaku sekarang: **GUI tidak mengendalikan apa pun yang memakan
waktu.** Ia memulai, lalu menonton.

## Unduhan bisa dilanjutkan

Berkas `.part` **tidak** dihapus saat unduhan terputus; percobaan berikutnya
meminta sisanya lewat header `Range`. Gagal di 100 MB tidak lagi berarti
mengulang 111 MB.

Satu jebakan yang dijaga uji: server yang MENGABAIKAN `Range` menjawab 200 dan
mengirim dari awal. Kalau itu ditambahkan ke berkas separuh, hasilnya berkas
rusak yang ukurannya justru terlihat wajar — kerusakan paling buruk, sebab tidak
kelihatan. Karena itu jawaban 200 memicu berkas lama dibuang lebih dulu.

## Beberapa cermin per komponen

`recipe.urls` adalah daftar, dicoba berurutan. Bukan kemewahan: gyan.dev —
sumber ffmpeg Windows yang lazim — **tidak terjangkau dari sebagian jaringan**
(terbukti dari Indonesia: TLS handshake timeout). Satu alamat mati berarti
komponen wajib tidak bisa dipasang, dan pengguna tidak punya jalan lain di dalam
aplikasi.

Karena itu ffmpeg Windows kini mengambil **cermin GitHub** lebih dulu
(`GyanD/codexffmpeg`, build yang sama persis dari orang yang sama), baru
gyan.dev, baru BtbN. Model whisper punya cermin kedua juga.

## Menunjuk program sendiri

Deteksi otomatis tidak akan pernah menang melawan semua cara orang memasang
program. Tombol **"Cari sendiri"** di tiap baris membuka pemilih berkas yang
sama dengan pemilih video, dan path-nya disimpan ke `.env`
(`CLIPPER_CHROME`, `CLIPPER_FFMPEG_BIN`, `CLIPPER_FFPROBE_BIN`,
`CLIPPER_WHISPER_BIN`) **sekaligus dipasang ke proses yang sedang jalan** —
tanpa itu, pengguna wajar mengira pilihannya tidak tersimpan.

Menyuruh pengguna aplikasi berjendela menyunting berkas `.env` adalah sisa dari
masa CLI.

## Browser untuk kartu berita

`capture.Find` dulu hanya mengenal biner Linux + path `/mnt/c/...` milik WSL.
Aplikasi yang jalan **asli di Windows** tidak pernah melihat Chrome atau Edge
yang jelas-jelas terpasang, dan menjawab "No browser found". Sekarang ia
menelusuri `%ProgramFiles%`, `%ProgramFiles(x86)%`, dan `%LOCALAPPDATA%` untuk
Edge, Chrome, Brave, Vivaldi, Chromium, Opera.

**Firefox tidak bisa dipakai**, dan itu bukan kelalaian: kartu dirender dengan
flag khas Chrome (`--headless=new`, `--screenshot`,
`--force-device-scale-factor`, `--dump-dom`). Yang dibutuhkan keluarga Chromium.

## Dari mana binernya diambil

Paket `internal/setup`. Resep per OS ada di `recipe.go`:

| Komponen | Windows | Linux | macOS |
| --- | --- | --- | --- |
| whisper.cpp | rilis resmi `whisper-bin-x64.zip` | `whisper-bin-ubuntu-x64.tar.gz` (arm64 juga) | tidak ada biner resmi → `brew install whisper-cpp` |
| ffmpeg | gyan.dev `ffmpeg-release-essentials.zip` | rilis statisnya `.tar.xz` → pakai paket sistem | evermeet.cx `.zip` |
| model whisper | HuggingFace `ggml-<nama>.bin` | sama | sama |

Tiga keputusan di baliknya:

- **Hanya zip dan tar.gz.** Keduanya ada di pustaka standar Go. Rilis `.tar.xz`
  (ffmpeg statis untuk Linux) sengaja tidak dipakai: menambah dependensi
  eksternal cuma untuk membongkarnya melanggar aturan "standard library saja".
  Linux memang sasaran uji cepat, bukan sasaran distribusi utama.
- **Versi whisper.cpp dipaku (`v1.9.1`), bukan "latest".** Rilis baru bisa
  mengubah nama berkas di dalam arsip, dan yang gagal karenanya adalah
  pemasangan di mesin pengguna — tempat paling buruk untuk menemukan kejutan.
- **Semua isi arsip diratakan ke satu folder** (`ToolsDir`). whisper.cpp membawa
  pustaka bersamanya, dan engine memang menunjuk `LD_LIBRARY_PATH` ke folder
  binernya. Symlink di dalam tar ikut dibuat ulang — rantai
  `libwhisper.so → .so.1 → .so.1.9.1` itu yang membuat binernya bisa jalan.

Selain itu: unduhan ditulis ke `.part` lalu diganti nama (berkas setengah jadi
tidak boleh terlihat seperti berkas jadi), biner diberi bit x sendiri (rilis
Windows tidak membawanya), dan satu komponen tidak bisa dipasang dua kali
sekaligus.

## Yang tidak dipasang engine

Ollama dan Chrome. Keduanya aplikasi utuh milik orang lain dengan pemasangnya
sendiri; yang bisa dilakukan hanyalah mendeteksi dan menunjukkan halaman
unduhnya. Ollama yang sudah jalan ikut dilaporkan model apa saja yang ada.

## Bukti jalannya

Dari folder `bin`/`models` yang benar-benar kosong, lewat API:

- `whisper.cpp` terunduh & dibongkar → `whisper-cli --help` jalan;
- model `tiny` terunduh (8 detik, 39 laporan progres);
- `clipper run` di folder itu menghasilkan klip 9:16 bersubtitle — jadi biner
  hasil unduhan benar-benar menggerakkan pipeline, bukan sekadar ada.

## Sisa pekerjaan

Tombol pasang ffmpeg belum bisa diuji di Linux (resepnya memang tidak ada di
sana). Perlu dicoba di Windows sebelum rilis pertama — bersama seluruh alur,
sebab Windows-lah sasaran utamanya.
