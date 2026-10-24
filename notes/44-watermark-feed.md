# 44 — Watermark: feed Instagram, banyak logo, kisi, kotak judul berita

Dikerjakan 18 Oktober 2026. Diminta: selain Shorts, halaman watermark juga
untuk **feed Instagram**; logo boleh **lebih dari satu**; **kisi** supaya
memindah logo gampang; dan judul gaya kartu berita TIMES Indonesia (kotak putih
sudut bulat, teks merah, logo di pojok kiri atas, baris "Foto: …") yang
**otomatis**, tapi tetap bisa ditambah logo lain.

Keputusan pemilik di sesi yang sama: feed menerima **video + foto**, rasionya
**4:5 dan 1:1**, judul **diketik sekali untuk semua berkas**.

## Bentuknya

- `config.Watermark.Format` = `short` | `feed45` | `feed11`. Ruang koordinat
  lebarnya tetap 1080, tingginya 1920 / 1350 / 1080 (`config.FrameSize`).
  PlayResY .ass dan `ffmpeg.Watermark.RefH` ikut format itu, jadi angka di
  pratinjau tetap = angka di hasil. Halaman klip tidak mengirim format = short.
- **Rasio lain DISESUAIKAN, tidak ditolak** — keputusan pemilik di hari yang
  sama ("tidak perlu ada salah rasio, tinggal masukkan gambarnya"), menggantikan
  aturan tolak di notes/41. Berkas yang rasionya sudah sama tetap seukuran
  aslinya; yang beda keluar seukuran bingkai format, lewat `Fit`: `crop`
  (bawaan, penuhi bingkai & potong tengah = reframe center/zoom 100) atau
  `whole` (seluruh gambar di atas salinan blur-nya = reframe fit/zoom 0). Jalur
  reframe yang sama dengan halaman klip, dan pratinjau `/api/frame` memakai
  pilihan yang sama, jadi potongannya terlihat sebelum jalan.
- **Foto** dikenali dari akhirannya (`watermark.IsImage`) dan lewat
  `ffmpeg.BurnImage`: rantai filter yang SAMA dengan video, `-frames:v 1`.
  PNG keluar PNG, sisanya JPEG. Waktu tampil diabaikan untuk foto.
- **Logo tambahan** = `Watermark.Logos`; `watermarkChain` ditumpuk berurutan
  dengan label bernomor (`[bvid0]`, `[blogo1]`, …). Logo pertama tetap di
  `Image/X/Y/Width/Height` supaya halaman klip & CLI lama tidak berubah.
- **Kotak judul** = `Headline.Box` + `Headline.Credit`, digambar libass
  (`subtitle.writeNewsBox`): kotak vektor `\p1` bersudut bulat, teks rata kiri,
  kredit kecil. Tingginya mengikuti teks, posisinya cuma naik-turun (`DY`).
  Tidak memakai Chrome/`card`: alasan notes/41 masih berlaku.
- **Lebar teks DIUKUR, bukan ditebak** (`internal/fontmetrics`: hmtx + cmap
  dari berkas font yang sama dengan libass). Tebakan per jenis huruf sempat
  dipakai dan teks huruf kecil panjang keluar dari kotak di pratinjau
  (dilaporkan pemilik). Kata tunggal yang lebih lebar dari kotak dipenggal per
  huruf. **Enter pengguna = baris baru.** Pratinjau MEMINTA tata letaknya ke
  engine (`POST /api/watermark/layout`) — tidak ada salinan rumus di GUI; hanya
  posisi tegaknya (dari DY) yang dihitung GUI supaya seret tetap mulus.
- **Sudut**: melingkar (bawaan) atau lancip (`Headline.Sharp`).
- **Outline**: pada judul biasa = tepi huruf; pada kotak judul = garis tepi
  KOTAK (`\bord` di gambar vektor). Tebal 0–12 + warna, bisa diubah.
- **Logo terkunci ke kotak judul** (jangkar ke-10): disimpan sebagai jarak tepi
  kiri & tepi bawah logo dari pojok kiri-atas kotak, jadi menggeser kotak
  membawa logonya. Mengunci tidak memindahkan logo. Saat diseret, logo
  MENEMPEL ke tepi kotak (jarak 24) dan diberi tanda magnet; logo terkunci
  bertanda gembok. **Otomatis**: menyalakan kotak judul mengunci logo pertama
  di tepi atasnya, rata kiri.
- **Template** (`/api/watermark/templates`, `data/templates/watermark.json`,
  satu daftar untuk semua akun): menyimpan format, cara sesuai rasio, logo,
  gaya judul, kredit, kisi — BUKAN berkas dan teks judul. Isinya milik GUI,
  engine cuma menyimpan. Logo disimpan sebagai path: di versi web logo unggahan
  tidak ikut tersapu (`sweepUploads` hanya membuang folder berisi video).

## Yang sengaja tidak dibuat

- ~~Logo menumpang ke dalam kotak.~~ **Dibalik 24 Oktober 2026**: hasil render
  menaruh kotak (.ass, dibakar terakhir) DI ATAS logo, sedangkan pratinjau HTML
  menaruh logo di atas kotak — logo terpotong kotak (dilaporkan pemilik). Kini
  `EncodeOpts.LogosOverSubs` (nyala saat kotak judul aktif) menumpuk logo
  SESUDAH subtitle. Bukti: logo ketiga di `TestRunFeedPhotoWithLogosAndNewsBox`
  yang menindih tengah kotak harus tetap hijau.
- **Judul per berkas.** Satu teks untuk satu job, sesuai keputusan di atas.
- Halaman klip tidak mendapat logo tambahan / kotak judul (pipeline belum
  meneruskan `Logos`); tambahkan di `pipeline` bila diminta.

## Kisi

`GridPicker` (sudah ada di `guides.tsx`) dipasang di bawah pratinjau: kerapatan
kisi + tombol "selalu tampil", bawaan 40 dan selalu tampil. Kisi yang sama
menempelkan seretan (Alt = lepas). `useLayerDrag` dan `Guides` kini menerima
tinggi bingkai (`playH`).

## Bukti

- `watermark.TestRunFitsOtherRatioIntoFrame` — 16:9 keluar 1080x1920, 9:16
  tetap seukuran aslinya.
- `watermark.TestRunFeedPhotoWithLogosAndNewsBox` — foto 4:5 + video 4:5 jadi,
  video 9:16 di mode feed keluar 1080x1350 dengan crop DAN whole, dan dari PIKSEL hasilnya: kotak putih di tempat
  yang dihitung `NewsBox`, logo utama merah, logo kedua hijau.
- Render GUI lewat CDP (1240x860): pratinjau dan hasil render memenggal judul
  di kata yang sama, logo pin di tepi atas kotak.

## Putaran berikutnya (18 Oktober 2026)

- **Warna**: teks kuning di kotak putih tidak terbaca (dilaporkan pemilik).
  Kotak judul kini punya **warna isi** sendiri (`Headline.BoxColor`), dan
  semua pilihan warna (teks, outline, kotak) = 6 swatch + pemilih warna bebas
  (#rrggbb, dibaca `assColor`). Swatch terpilih diberi centang — bingkai tipis
  di swatch putih/kuning tidak kelihatan. Baris kredit otomatis terang di
  kotak gelap (`creditColor`, ambang luminans 140, sama di GUI).
- **Kisi 5 & 10**: tetap menempel per 5/10, digambar dua lapis — garis tipis
  per kisi + garis tegas per kelipatan >= 20 (di pratinjau 340 px kotak 5
  cuma 1–2 px).
- **Pratinjau hasil** (`POST /api/watermark/preview` → `watermark.Preview`):
  satu frame berkas pertama dibakar lewat `one()` yang SAMA dengan job, jadi
  bukan tiruan HTML. Waktu tampil diabaikan (logo & judul selalu terlihat).
- **Perataan** kotak judul (`Headline.Align`: left | center | right): judul
  DAN kredit memakai jangkar `\an7/8/9` di tepi kiri isi, tengah kotak, atau
  tepi kanan isi. Hanya untuk kotak judul — judul biasa memakai satu jangkar
  `\an5`, dan .ass tidak bisa meratakan baris kiri sambil menengahkan bloknya.
- **Kode warna** selain RGB: kolom yang menerima HEX (dengan/tanpa #), rgb(),
  hsl(), hwb(), nama warna CSS, dan cmyk(c,m,y,k). Semuanya diterjemahkan GUI ke
  #rrggbb (CMYK dihitung sendiri, sisanya lewat kanvas browser) — engine tetap
  hanya membaca nama dasar atau #rrggbb. Kode yang bukan warna ditandai merah.
