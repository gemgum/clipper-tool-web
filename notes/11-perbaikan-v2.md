# 11 — Perbaikan v2 (flow pengembangan)

Empat catatan dari uji pakai MVP, beserta akar masalah yang sudah diverifikasi
di kode dan di output job `data/job_0002_2026-07-28_02-18-04`.

## Ringkasan temuan

| # | Keluhan | Akar masalah |
|---|---|---|
| 1 | Belum ada pembatas UI tiap sosmed | Fitur belum ada di GUI |
| 2 | Subtitle "tunggal" jadi kuning & tak jalan | `subtitle.go:37` memaksa kuning; `{\k}` hanya menyapu warna, bukan mode 1 kata; timing per kata dibagi rata |
| 3 | Subtitle terlalu cepat pada teks panjang | `subtitle.go:181` membagi halaman proporsional tanpa durasi minimum (ditemukan halaman 0,45 dtk) |
| 4 | Pemotong selalu ~30 dtk (maks 36) | `segment.go:31` memotong begitu durasi ≥ `targetMin`; preset `auto` = min 30 dtk |

Bukti #4 — durasi 15 klip job terakhir: 30, 31, 31, 32, 32, 32, 32, 33, 34, 34,
35, 36, 37, 30, 30 detik. Tidak ada variasi karena selalu berhenti di ambang bawah.

Bukti #2/#3 — potongan `clip_01.ass`:

```
Style: Default,Montserrat,84,&H0000FFFF,...   ← kuning dipaksa
Dialogue: 0:00:04.00,0:00:07.55,...{\k45}Hari-hari {\k45}aku ... (3 baris/3,5 dtk)
Dialogue: 0:00:07.55,0:00:08.00,...{\k44}itu.                     ← tampil 0,45 dtk
```

## Status: SEMUA TAHAP SELESAI & TERUJI (28 Juli 2026)

Bukti uji akhir (`clipper run` 5 menit, model base, preset 60, mode karaoke,
`-save both`):

| Yang diuji | Sebelum | Sesudah |
|---|---|---|
| Durasi klip | 30–37 dtk (semua) | 60 / 59 / 60 dtk (ikut preset) |
| Tampilan subtitle terpendek | 0,45 dtk | 1,23 dtk (rata 2,0–2,6 dtk) |
| Warna dasar saat karaoke | dipaksa kuning | putih (pilihan pengguna), sorot kuning per kata |
| Mode satu kata | tidak ada | 1 kata per tampilan, ikut timestamp asli |
| Berkas per klip | 1 | `clip_01.mp4` + `clip_01_polos.mp4` + `clip_01.srt` |

Uji otomatis: `engine/internal/segment/segment_test.go` (durasi kandidat) dan
`engine/internal/subtitle/subtitle_test.go` (tak ada tampilan kilat, kata tidak
hilang, warna tidak dipaksa, mode word/karaoke).

## Keputusan desain

- Gaya subtitle jadi **tiga pilihan**: `normal` · `karaoke` (highlight per kata) ·
  `word` (satu kata per layar).
- Warna utama **selalu** ikut pilihan pengguna. Warna sorot jadi setelan
  terpisah (default kuning).
- Pembatas sosmed = **panduan di preview + preset posisi**, tidak ikut ter-render.

## Urutan pengerjaan

```
Tahap 0 (fondasi)  ──┬──> Tahap 2 (gaya subtitle)
timestamp per kata   └──> Tahap 3 (kecepatan subtitle)

Tahap 1 (durasi klip)      ← independen, bisa duluan
Tahap 4 (pembatas sosmed)  ← independen, GUI saja
Tahap 5 (verifikasi end-to-end)
```

---

### Tahap 0 — Timestamp per kata (fondasi #2 & #3)

`engine/internal/transcribe/whisper.go`, `types/types.go`

- Tambah flag `-ojf` (`--output-json-full`) + `-sow` pada pemanggilan whisper-cli
  → JSON memuat array `tokens` beserta `offsets` tiap token. Satu kali jalan,
  tanpa tambahan waktu transkripsi.
- Parse token → `TranscriptSegment.Words []Word{Start, End, Text}`; buang token
  khusus (`[_BEG_]`, `[_TT_..]`) dan gabungkan sub-token menjadi kata utuh.
- Fallback: bila `Words` kosong (model/JSON lama), bagi rata seperti sekarang —
  semua kode di bawah harus tetap jalan tanpa word timing.
- Opsional lanjutan: `-dtw <model>` untuk timestamp yang jauh lebih presisi
  (biaya CPU kecil). Ditaruh di belakang opsi, tidak default.

**Selesai bila:** transkrip JSON internal punya `words` dengan waktu masuk akal
(cek manual 1 segmen vs audio).

### Tahap 1 — Durasi klip (#4)

`engine/internal/segment/segment.go`, `config/config.go`

- Kenalkan **durasi ideal** = titik tengah rentang (mis. auto → 60–90 dtk).
  Jangan potong hanya karena sudah lewat `targetMin`.
- Aturan potong baru, berurutan:
  1. Kumpulkan segmen sampai `d >= ideal`.
  2. Dari situ, potong di akhir kalimat **pertama** yang ditemukan, selama
     `d <= targetMax`.
  3. Bila lewat `targetMax` tanpa akhir kalimat → mundur ke akhir kalimat
     terakhir, sisanya dibawa ke kandidat berikutnya (perilaku sekarang, tetap).
  4. Bonus: jeda diam panjang (>1,5 dtk antar segmen) jadi titik potong yang
     lebih disukai daripada sekadar tanda titik.
- Ubah preset `auto` dari `30–180` menjadi `45–120` dengan ideal 70, supaya
  label GUI ("auto") benar-benar menghasilkan variasi.
- Terapkan juga ke jalur LLM: momen dari Claude/Ollama yang durasinya di bawah
  `targetMin` digabung/diperluas ke batas segmen tetangga sebelum di-render.

**Selesai bila:** satu video uji menghasilkan durasi klip yang bervariasi dan
mengikuti preset (preset 60 → mayoritas 48–75 dtk, bukan 30-an).

### Tahap 2 — Gaya subtitle & warna (#2)

`config/config.go`, `subtitle/subtitle.go`, `gui/app/page.tsx`

- `config.Subtitle`: ganti `Karaoke bool` → `Mode string` (`normal|karaoke|word`)
  \+ `HighlightColor string`. Pertahankan pembacaan `karaoke: true` lama sebagai
  alias `karaoke` supaya preset lama tidak rusak.
- `styleColors()`: **hapus** pemaksaan kuning. `PrimaryColour` selalu = warna
  pengguna.
- Karaoke TIDAK memakai tag `\k` (yang menyapu warna permanen dan membuat
  seluruh baris berakhir kuning). Sebagai gantinya: satu `Dialogue` per kata,
  seluruh tampilan tetap warna dasar, hanya kata aktif dibungkus `{\c…&}` —
  hasilnya "kata aktif disorot", persis yang diminta.
- Renderer baru `renderWordMode()`: satu `Dialogue` per kata memakai
  `Words[i].Start/End` (fallback bagi rata), teks 1 kata, ukuran font boleh
  dinaikkan otomatis (mode ini biasanya lebih besar).
- GUI: ganti checkbox "Karaoke per-kata" jadi dropdown **Gaya subtitle**
  (Normal / Highlight per-kata / Satu kata per layar) + pemilih **warna sorot**
  yang hanya muncul untuk karaoke/word. Preview ikut mode (jangan lagi memaksa
  `#ffdd00` di `page.tsx:499`).

**Selesai bila:** pilih putih → hasil putih; mode `word` benar-benar menampilkan
satu kata bergantian; mode `karaoke` menyorot kata aktif tanpa mengubah warna dasar.

### Tahap 3 — Kecepatan subtitle (#3)

`subtitle/subtitle.go`, `config`, GUI

- Batas kata per halaman dihitung dari **kecepatan baca**, bukan hanya lebar
  baris: `maxChars ≈ cps × durasiHalaman` dengan `cps` ±14 char/dtk (bisa diatur).
- **Durasi minimum halaman** 1,2 dtk. Bila halaman lebih pendek:
  - perpanjang ke jeda kosong sebelum segmen berikutnya (bila ada), atau
  - gabung ke halaman sebelumnya/berikutnya (hilangkan halaman "itu." 0,45 dtk).
- Batas halaman memakai `Words` (Tahap 0) supaya waktu tampil = waktu ucap
  sebenarnya, bukan proporsi jumlah kata.
- GUI: setelan **Kecepatan subtitle** (Lambat / Normal / Padat) yang memetakan ke
  `cps` + `minDur`, dan opsi maksimum baris (2 atau 3).

**Selesai bila:** tidak ada `Dialogue` di bawah 1,0 dtk pada video uji, dan teks
panjang terbaca nyaman saat ditonton.

### Tahap 4 — Pembatas area aman sosmed (#1)

`gui/app/page.tsx`, `gui/app/globals.css` (GUI saja, tanpa perubahan engine)

- Dropdown **Platform**: TikTok · Instagram Reels · YouTube Shorts · Umum · Mati.
- Overlay zona tak-aman di preview 9:16 (persen terhadap tinggi/lebar frame):

  | Platform | Atas | Bawah | Kanan |
  |---|---|---|---|
  | TikTok | 8% | 20% | 16% |
  | Reels | 7% | 17% | 15% |
  | Shorts | 6% | 13% | 14% |
  | Umum | 8% | 20% | 16% |

  (angka awal, dikalibrasi dengan screenshot asli tiap aplikasi)
- Zona digambar sebagai arsiran gelap + label kecil ("caption", "tombol aksi").
- Tombol **"Taruh di area aman"** → set `subY` ke tengah area aman (dan `subX`
  bergeser kiri bila zona kanan aktif).
- Peringatan halus bila posisi subtitle sekarang masuk zona tak-aman.
- Platform ikut tersimpan di preset `localStorage` bersama setelan lain.

**Selesai bila:** ganti platform → arsiran berubah, tombol memindahkan subtitle
ke posisi yang tidak tertutup UI.

### Tahap 6 — Opsi simpan klip (permintaan tambahan)

`config`, `pipeline`, `ffmpeg`, `api`, GUI

- `Options.SubtitleOutput`: `burn` (default, bersubtitle) · `clean` (klip polos)
  · `both` (dua berkas: `clip_01.mp4` + `clip_01_polos.mp4`).
- Mode `clean`/`both` sekalian menulis `clip_01.srt` di folder output, supaya
  klip polos bisa disubtitle ulang di CapCut/Premiere.
- API: `GET /api/jobs/{id}/clips/{clip}/file?varian=polos|srt`.
- GUI: dropdown "Simpan klip" + tautan unduh terpisah (bersubtitle / polos / .srt).
- CLI: `-save burn|clean|both`.

Catatan biaya: mode `both` meng-encode dua kali → waktu render ±2x.

### Tahap 5 — Verifikasi

- Render ulang 1 video uji (±10 menit, model `base`) untuk tiap kombinasi:
  preset durasi 60 + gaya normal/karaoke/word.
- Cek: distribusi durasi klip, tidak ada halaman subtitle <1 dtk, warna sesuai,
  subtitle di dalam area aman platform terpilih.
- Perbarui `notes/08-status-mvp.md`.

## Preview & font di GUI (ronde perbaikan setelah uji pakai)

Semuanya di `gui/app/page.tsx` + endpoint font baru di engine.

**Preview**
- Video baru masuk (unggah/seret/ketik path) → preview lama direset lalu dimuat
  sendiri, ditunda 500 ms agar path yang sedang diketik tidak memicu probe tiap
  huruf. Ditambah tombol muat ulang manual & reset.
- URL frame membawa `&n=<nonce>` yang naik tiap muat ulang. Tanpa itu, video
  yang ditimpa di path sama akan menampilkan gambar lama dari cache browser —
  header `no-cache` saja tidak cukup untuk `<img>` ber-URL identik.

**Menggeser subtitle**
- Selisih titik pegang disimpan saat pointer ditekan lalu dijumlahkan tiap
  gerakan. Sebelumnya posisi disamakan langsung dengan kursor sehingga teks
  melompat begitu disentuh — itu sumber rasa "kurang stabil".
- Magnet ke garis tengah, toleransi 20 unit (≈5 piksel layar pada preview).
- Garis tengah X, Y, dan penanda titik tengah XY muncul saat digeser; ada
  centang untuk mengunci agar selalu tampil. Murni panduan — tidak ikut render.

**Bug "posisi aman terlalu ke kiri"**: `placeSafe` dulu menengahkan X ke lebar
sisa di luar kolom tombol kanan (`1080 × 0,84 / 2 = 453`). Padahal subtitle
dirender rata tengah terhadap titik itu, jadi hasilnya miring 87 unit ke kiri.
Sekarang X selalu 540; hanya Y yang menyesuaikan zona bawah.

**Font manual**: `GET /api/font-check?name=` memvalidasi format nama (regex
ketat, karena nama ini masuk ke .ass dan ke argumen `fc-match`) lalu mencari
fontnya — bawaan proyek dulu, baru fontconfig. `fc-match` selalu mengembalikan
sesuatu, jadi family hasilnya wajib dibandingkan dengan yang diminta; kalau
berbeda berarti font tidak terpasang dan libass akan diam-diam menggantinya.
`font-file` ikut melayani font sistem yang lolos cek supaya preview memakai
font asli, dan job ditolak di GUI bila font manual belum valid.

Sekalian diperbaiki: daftar font dari `/api/fonts` dulu menimpa font pilihan
dari preset (daftar datang belakangan), jadi pilihan font selalu balik ke font
pertama tiap halaman dimuat ulang.

## Preview mengikuti mode reframe (2b)

**Cacatnya**: `ExtractFrame` selalu `increase` + `crop`, jadi preview selalu
menampilkan versi crop tengah walaupun pengguna memilih "muat utuh". Render-nya
sendiri sudah benar sejak awal — yang salah cuma preview.

Akibatnya untuk sumber 16:9 ke kanvas 9:16: render mode `fit` menaruh video
sebagai pita setinggi **81/256 ≈ 32%** kanvas dengan latar blur di atas-bawah,
sedangkan preview menampilkannya diperbesar ±3,2× memenuhi kanvas. Subtitle yang
ditaruh "di bawah dagu" saat preview mendarat di area latar blur saat dirender.

**Perbaikan**: rantai filter diangkat ke satu fungsi bersama
`ffmpeg.ReframeFilter(mode, w, h)` yang dipakai `ClipReframe` (render) **dan**
`ExtractFrame` (preview). Ini inti perbaikannya — cacat tadi muncul justru
karena kedua tempat menyusun filternya sendiri-sendiri, jadi ketika mode `fit`
ditambahkan ke render, preview tertinggal tanpa ada yang sadar.

`/api/frame` menerima param `reframe` (default center) dan GUI mengirim mode
yang sedang dipilih, jadi preview = satu frame dari klip jadi, lengkap dengan
latar blur-nya. Blur di preview murah karena preview dipaksa 720p (720×1280),
bukan 1080×1920. Ruang koordinat tidak berubah: `subX`/`subY`, zona aman, dan
garis tengah tetap sah.

**face_follow ditandai belum tersedia, bukan diperlakukan sebagai center.**
`config.Reframe.Cek()` menolaknya, dipanggil dari `Options.Validate()` (jadi
CLI & pembuatan job berhenti di depan) dan dari `/api/frame`. Di GUI opsinya
tampil tapi `disabled`. Sebelumnya `enc.Mode == "fit"` yang bernilai false
membuat face_follow diam-diam dirender sebagai center — persis penggantian
senyap yang dilarang notes/12.

Diuji: frame `center` vs `fit` diambil dari video uji 1280×720 lewat endpoint —
`center` menampilkan potongan tengah yang di-zoom, `fit` menampilkan frame utuh
sebagai pita tengah di atas latar blur. `face_follow` & mode ngawur dijawab 400
dengan pesan yang jelas; CLI `-reframe face_follow` berhenti sebelum pipeline.

## Diagnostik ffmpeg & tata letak berkas kerja

Dipicu satu job nyata yang gagal di klip ke-14 (13 klip sudah jadi) dengan pesan
`clip+reframe gagal: exit status 254` yang isinya cuma statistik x264.

**1. Pesan galat mengambil baris yang bermakna.** `run()` dulu memotong 500
karakter TERAKHIR stderr — dan ekor keluaran ffmpeg selalu blok statistik x264,
bagian paling tidak berguna, sementara baris sebabnya tercetak jauh lebih awal
dan ikut terbuang. `ringkasGalat()` kini menyaring baris ber-penanda (`error`,
`no such`, `permission`, `no space`, dst), mengambil maksimal 3 yang terakhir,
lalu `petunjukGalat()` menambahkan tindak lanjut dalam bahasa Indonesia.

Kode keluar ffmpeg ternyata negatif errno: **254 = ENOENT** (256−2), 243 =
EACCES, 234 = EINVAL. Jadi 254 pada kasus itu berarti folder keluaran hilang di
tengah render — bukan kerusakan video atau kesalahan filter.

**2. Folder keluaran dijamin ada sebelum tiap klip.** `ClipReframe` menjalankan
`MkdirAll(filepath.Dir(out))` di awal. Render satu job bisa puluhan menit; kalau
foldernya sempat terhapus atau dipindah, dulu seluruh hasil encode terbuang.
Dipasang di ffmpeg (bukan pipeline) supaya berlaku untuk semua pemanggil.

**3. Berkas kerja dipisah ke `tmp/`, `.ass` dihapus setelah dibakar.** Bila
pengguna tidak mengisi folder keluaran, `outDir = workDir` — jadi `audio.wav`,
`transcript.json`, dan `clip_XX.ass` dulu menumpuk bersama klip final. Sekarang
ketiganya di `workDir/tmp/`, dan `.ass` hanya dibuat bila memang akan dibakar
lalu dihapus setelah ffmpeg sukses (kalau gagal, sengaja ditinggal untuk
penelusuran). Komentar lama menyebut `.ass` "file sementara" padahal tak pernah
ada yang menghapusnya.

Ikut dihapus: field `subtitle_path` di `types.Clip` — tidak dipakai GUI maupun
API, dan sekarang akan menunjuk berkas yang sudah tiada. `notes/07` disesuaikan.

Diuji end-to-end dengan potongan 90 detik video Indonesia:
- mode `burn` → folder job berisi `clip_01.mp4` + `tmp/`; nol berkas `.ass`
- mode `both` → `clip_01.mp4`, `clip_01_polos.mp4`, `clip_01.srt` + `tmp/`
- folder tujuan sengaja dihapus → render tetap berhasil (folder dibuat ulang)
- folder hanya-baca → `izin tulis ditolak untuk folder keluaran`
