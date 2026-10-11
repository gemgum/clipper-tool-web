# Aplikasi desktop & pemasangan engine sekali klik

Sasaran berikutnya, ditetapkan 4 Agustus 2026.

## Bentuk yang dituju

Pengguna membuka satu berkas aplikasi. Kalau ada komponen yang belum ada —
whisper, model, Ollama — muncul tombol untuk memasangnya. Tidak ada terminal,
tidak ada `setup.sh`, tidak ada cmake.

Itu perbedaan pokoknya dengan keadaan sekarang: hari ini pemasangan adalah
pekerjaan pengembang; besok ia harus jadi pekerjaan aplikasi.

## Yang sudah diputuskan, jangan dibuka lagi

- Desktop tiga OS. `.exe` Windows sasaran utama, AppImage Linux untuk uji cepat,
  macOS menyusul. **Mode web ter-deploy ditolak.** Ditegaskan ulang 6 Agustus
  2026: membuka lewat browser hanya sah sebagai ALAT UKUR pengembangan
  (`scripts/measure-ui.mjs`), bukan bentuk produk. Deploy, multi-pengguna,
  alamat publik, dan tata letak ponsel tetap di luar cakupan. Lihat CLAUDE.md
  bagian "Sasaran: DESKTOP. Titik."
- Engine tetap Go, UI tetap Next.js, komunikasi tetap REST + SSE.
- Worker C++ sudah dibuang (selesai 4 Agustus 2026, lihat `20-buang-worker-cpp.md`).
  Build tiga OS sekarang cukup `go build`.

## Yang belum diputuskan

Shell jendelanya. Dua kandidat, keduanya masih hidup:

- **Chrome `--app=`** — nol dependensi tambahan, dan pencari browser sudah ada
  di `capture.go` untuk kartu berita.
- **Tauri + Go sidecar** — pengemasan, penandatanganan, dan notarisasi paling
  matang.

Tidak mendesak: fondasinya sama untuk keduanya, jadi pekerjaan di bawah ini
tidak perlu menunggu keputusan itu.

## Tiga penghalang yang sudah diketahui

Ada di kode sekarang dan menghalangi bentuk desktop apa pun:

1. ~~**Folder data ditulis di sebelah biner.**~~ **Selesai 4 Agustus 2026** —
   lihat bagian "Folder data per pengguna" di bawah.
2. ~~**Port 8787 dipaku, CORS `*`, tanpa token.**~~ **Selesai 4 Agustus 2026** —
   port acak + kunci sesi + CORS hanya-lokal, lihat `26-kunci-sesi.md`.
3. ~~**Berkas lokal disalin lewat HTTP.**~~ **Selesai 4 Agustus 2026** — GUI
   punya pemilih berkas sendiri dan berkas yang di-drop dicari di tempat, lihat
   `24-berkas-lokal.md`.

Ketiganya sudah tidak menghalangi. Yang tersisa untuk bentuk desktop tinggal
shell jendelanya sendiri.

## Folder data per pengguna (penghalang 1 — selesai)

`config.Layout` (engine/internal/config/layout.go) memetakan folder engine, dan
`config.Locate` memilih salah satu dari dua bentuk:

| | checkout sumber | terpasang |
| --- | --- | --- |
| data (job, cache, kartu, unggahan) | `<repo>/data` | `<data pengguna>/data` |
| model whisper | `<repo>/models` | `<data pengguna>/models` |
| biner (whisper-cli, ffmpeg) | `<repo>/bin` | `<data pengguna>/bin` |
| `.env` | `<repo>/.env` | `<data pengguna>/.env` |
| font | `assets/fonts` di sebelah biner, cadangan `<repo>/assets/fonts` | sama |

`<data pengguna>` = `%LOCALAPPDATA%\Clipper` · `~/Library/Application Support/Clipper`
· `$XDG_DATA_HOME/clipper` (bawaan `~/.local/share/clipper`).

Tiga keputusan yang menjelaskan bentuk di atas:

- **Penandanya `engine/go.mod`, bukan flag.** Berkas itu hanya ada di pohon
  sumber. Kalau pemilihannya harus diminta lewat flag atau env, mode terpasang
  akan selalu salah bagi pengguna yang tidak tahu harus memintanya. Efek
  sampingnya yang disengaja: dari checkout **tidak ada yang berubah** — model
  466 MB dan cache transkrip yang sudah ada tetap terpakai, jadi tidak perlu
  migrasi.
- **`ModelsDir`/`ToolsDir` dipisah dari `DataDir`.** Keduanya berisi barang
  unduhan yang boleh dibuang tanpa kehilangan hasil kerja, dan halaman
  Requirements butuh tempat pasti untuk menaruh unduhannya.
- **ffmpeg/ffprobe unduhan menang atas PATH.** Di mesin pengguna desktop PATH
  biasanya kosong dari keduanya; kalau pun ada, versinya tak terkendali.
  Sebaliknya, saat belum ada unduhan engine tetap memakai PATH — jalur
  pengembang hari ini tidak ikut mati.
- **LOCALAPPDATA, bukan APPDATA.** Isinya model ratusan MB sampai beberapa GB;
  folder roaming ikut disalin antar mesin di jaringan kantor.

Timpaan env tetap ada dan menang atas keduanya: `CLIPPER_DATA_DIR`,
`CLIPPER_MODELS_DIR`, `CLIPPER_TOOLS_DIR`, `CLIPPER_FONTS_DIR`,
`CLIPPER_ENV_FILE` — jalan keluar bila model harus ditaruh di disk lain.

`clipper serve` mencetak folder datanya di banner, ditandai `(source checkout)`
bila sedang memakai bentuk pertama. Dua bentuk itu tidak dipilih dengan flag,
jadi baris itulah satu-satunya cara tahu yang mana yang berlaku.

`Layout.Ensure()` membuat ketiga folder tulis di awal proses, bukan saat berkas
pertama ditulis: kegagalan izin harus muncul saat start, bukan sebagai job yang
mati di tengah setelah pengguna menunggu setengah jam.

## Pemasangan engine sekali klik

Bagian baru, dan yang paling menentukan rasanya sebagai aplikasi.

Sekarang `setup.sh` mengerjakan empat hal: membangun whisper.cpp dari sumber
(butuh cmake + g++), mengunduh model, mengunduh font. Tidak satu pun boleh
dituntut dari pengguna desktop.

Per komponen:

| Komponen | Rencana |
| --- | --- |
| whisper | Jangan bangun dari sumber. Pakai rilis biner resmi whisper.cpp per OS, unduh saat dibutuhkan. |
| model whisper | Unduh dari HuggingFace saat pengguna memilihnya, dengan progres. `small` ≈ 466 MB. |
| ffmpeg | Wajib ada. Pilihannya: bundel biner statis, atau unduh sekali di pemakaian pertama. |
| Ollama | Aplikasi terpisah, tidak bisa dibundel. Deteksi keberadaannya; kalau tidak ada, tombol yang membuka halaman unduhnya. Kalau ada, deteksi model dan tawarkan `ollama pull`. |
| font subtitle | Kecil. Bundel saja, jangan diunduh. |
| Chrome | Sudah ditangani `capture.go`. Hanya perlu bagi tab kartu berita. |

Bentuk antarmukanya: satu halaman **Requirements** berisi daftar komponen
dengan status (ada / belum ada / sedang mengunduh) dan tombol per baris.

**Sudah ada, 4 Agustus 2026.** Halaman Requirements jadi tab ketiga di GUI;
engine memasang whisper.cpp, ffmpeg, dan model whisper sendiri. Rinciannya —
dari mana binernya diambil dan kenapa — di `25-pemasangan-komponen.md`.

## Yang sudah dikerjakan

| | Status | Catatan |
| --- | --- | --- |
| Folder data per pengguna | selesai 4 Agu 2026 | di berkas ini, bagian di atas |
| Path lokal, bukan unggahan | selesai 4 Agu 2026 | `24-berkas-lokal.md` |
| Halaman Requirements + pemasangan | selesai 4 Agu 2026 | `25-pemasangan-komponen.md` |
| Port acak + kunci sesi | selesai 4 Agu 2026 | `26-kunci-sesi.md` |

## Berikutnya

1. **Pilih shell jendelanya** — sekarang tinggal itu. Keduanya hanya perlu
   membaca `engine.json` lalu membuka GUI dengan `?token=…`, jadi pilihannya
   sudah tidak menyandera pekerjaan lain.
2. **Kemas GUI-nya.** Next.js masih dijalankan `npm run dev`; aplikasi butuh
   halaman statis yang disajikan engine, atau dibungkus shell.
3. **Coba seluruh alur di Windows.** Itu sasaran utamanya, dan resep pemasangan
   ffmpeg/whisper untuk Windows belum pernah dijalankan sungguhan.
