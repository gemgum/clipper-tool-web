# DESIGN.md — Video Clips

Spesifikasi desain UI untuk tool pemotong video 9:16 bersubtitle.
Dokumen ini adalah acuan implementasi: prinsip, struktur layar, token visual,
komponen, state, dan aturan penulisan teks.

- **Status:** usulan redesign (v1)
- **Tanggal:** 9 Oktober 2026
- **Target pengguna:** tim non-teknis (operator konten), bukan engineer
- **Mockup interaktif:** https://claude.ai/artifact/MrYFNzCGJ8N5Cs9DjRQyx9

---

## 1. Masalah pada UI saat ini

| #   | Masalah                                                                                           | Dampak                                                                  |
| --- | ------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| 1   | 25+ kontrol tampil rata di satu layar tanpa hierarki                                              | Operator tidak tahu mana yang wajib disentuh, mana yang boleh dibiarkan |
| 2   | Pratinjau 9:16 ditampilkan kecil di pojok kiri                                                    | Bagian paling menentukan hasil justru paling sulit dilihat              |
| 3   | Setelan teknis (Whisper model, engine, FPS, resolusi, outline, zoom) sejajar dengan setelan dasar | Beban kognitif besar; risiko salah ubah                                 |
| 4   | Posisi subtitle diisi koordinat mentah `540·1440`                                                 | Tidak bermakna bagi non-teknis                                          |
| 5   | Status proses hanya teks kecil "Running · 24%" + log hitam di dasar halaman                       | Tidak jelas sedang apa, berapa lama lagi                                |
| 6   | Tidak ada area hasil sama sekali                                                                  | Padahal hasil klip adalah alasan tool ini dipakai                       |
| 7   | Label berbahasa campur (Fit, Pacing, Outline, Safe area)                                          | Perlu pelatihan untuk setiap istilah                                    |

---

## 2. Prinsip desain

1. **Keputusan dulu, setelan belakangan.** Layar utama hanya memuat empat
   keputusan yang benar-benar diambil operator tiap kali bekerja.
2. **Default yang benar lebih baik daripada opsi yang lengkap.** Setiap setelan
   teknis punya nilai bawaan yang sudah teruji; kalau 90% pemakaian tidak
   mengubahnya, setelan itu masuk laci lanjutan.
3. **Pratinjau adalah sumber kebenaran.** Setiap perubahan gaya langsung terlihat,
   tanpa tombol "terapkan".
4. **Istilah manusia, bukan istilah mesin.** Tidak ada singkatan teknis di layar
   utama. Koordinat diganti pilihan platform.
5. **Proses panjang harus jujur.** Tampilkan tahap, progres, dan perkiraan sisa
   waktu; hasil parsial muncul begitu siap.
6. **Satu aksi utama per layar.** Hanya satu tombol biru di tiap layar.

---

## 3. Struktur aplikasi

Tiga layar, satu alur linear:

```
[1] Atur klip  ──Buat klip──▶  [2] Sedang diproses  ──selesai──▶  [3] Hasil klip
      ▲                                                                  │
      └──────────────────── Buat batch baru ─────────────────────────────┘
```

Tidak ada navigasi samping. Header tetap sama di ketiga layar (identitas app +
nama file sumber), hanya aksi di kanan yang berbeda.

---

## 4. Layar 1 — Atur klip

### 4.1 Tata letak

```
┌─ Header ─────────────────────────────────────────────────────────┐
│ Video clips                  [Pengaturan lanjutan] [Buat 15 klip]│
└──────────────────────────────────────────────────────────────────┘
┌─ Kolom kiri (flex 999 1 560px) ──┐ ┌─ Kolom kanan (flex 1 1 420px)┐
│ 1. Video sumber                  │ │ Pratinjau 9:16 (sticky)      │
│ 2. Gaya subtitle                 │ │ · scrubber + waktu           │
│ 3. Mau diposting ke mana         │ │ · toggle area tertutup UI    │
│ 4. Hasil yang diinginkan         │ │ · catatan posisi otomatis    │
│ ▸ Pengaturan lanjutan (tertutup) │ │                              │
└──────────────────────────────────┘ └──────────────────────────────┘
```

Dua kolom dibuat dengan `flex-wrap`, bukan grid tetap — di layar sempit kolom
pratinjau turun ke bawah tanpa kode responsif tambahan.

### 4.2 Isi tiap kartu

**Kartu 1 — Video sumber**
Thumbnail, nama file, durasi, resolusi, bahasa yang terdeteksi, tombol "Ganti video".
Path file sistem (`/var/www/clipper/data/uploads/...`) tidak ditampilkan; cukup nama file.

**Kartu 2 — Gaya subtitle**
Tiga preset sebagai kartu pilihan, bukan dropdown:

| Preset     | Perilaku                                     |
| ---------- | -------------------------------------------- |
| Polos      | Teks putih, garis tepi hitam                 |
| Sorot kata | Kata yang sedang diucapkan diberi blok warna |
| Karaoke    | Kata yang sedang diucapkan berubah warna     |

Di bawahnya hanya dua kontrol: **Ukuran huruf** (segmented: Kecil / Sedang / Besar)
dan **Warna sorotan** (4 swatch). Font, outline width, pacing, dan ukuran piksel
pindah ke laci lanjutan.

> Pemetaan ke nilai teknis: Kecil = 72px, Sedang = 94px, Besar = 120px pada
> kanvas 1080×1920. Outline mengikuti ukuran (≈ size/9), tidak diekspos.

**Kartu 3 — Mau diposting ke mana**
Tiga pilihan: TikTok / Reels / Shorts. Pilihan ini yang menentukan posisi subtitle
dan tinggi area aman atas-bawah. Operator tidak pernah mengetik koordinat.

| Platform | Area tertutup atas | Area tertutup bawah |
| -------- | ------------------ | ------------------- |
| TikTok   | 12%                | 24%                 |
| Reels    | 10%                | 20%                 |
| Shorts   | 8%                 | 16%                 |

**Kartu 4 — Hasil yang diinginkan**
Jumlah klip (stepper, 1–40), panjang tiap klip (Otomatis / Pendek 15–30s /
Sedang 30–60s), checkbox "Simpan juga versi tanpa subtitle".

**Laci — Pengaturan lanjutan** (`<details>`, tertutup secara bawaan)
Model transkrip (Whisper), engine & model kustom, koreksi transkrip dengan AI,
nama & istilah khusus, resolusi, kualitas, FPS, outline, zoom, fit, background,
watermark, posisi manual.

### 4.3 Pratinjau

- Rasio 9:16, lebar maksimal 372px, selalu terlihat saat scroll (`position: sticky`).
- Overlay garis miring menandai area yang tertutup UI aplikasi target.
- Contoh subtitle tiga kata, kata tengah dalam state "sedang diucapkan" agar
  perbedaan preset langsung terlihat.
- Scrubber dengan waktu berjalan (belum fungsional di mockup).

---

## 5. Layar 2 — Sedang diproses

Menggantikan indikator "Running · 24%" dan log hitam.

- **Judul besar + persentase**, progress bar penuh lebar.
- **Perkiraan sisa waktu** dalam bahasa biasa: "sekitar 9 menit lagi".
- **Tiga kartu tahap**, satu aktif: Transkrip → Pilih momen → Render video.
  Tiap tahap punya satu kalimat penjelas, bukan nama fungsi internal.
- **Klip yang sudah jadi** muncul bertahap sebagai thumbnail, dengan hitungan
  "3 dari 15". Operator bisa mulai mengunduh sebelum semuanya selesai.
- **Ringkasan pengaturan yang dipakai** di kolom kanan, plus tautan
  "Ubah pengaturan dan ulangi".
- **Catatan teknis** (log mentah) dilipat dalam `<details>`.
- Aksi: "Jalankan di latar belakang" dan "Batalkan" (destruktif, teks merah,
  bukan tombol merah penuh).

---

## 6. Layar 3 — Hasil klip

- **Banner hijau** ringkas: jumlah klip, jaminan kualitas ("teksnya di luar area
  tombol"), total ukuran file.
- **Aksi utama** "Unduh semua (15)" di header.
- **Filter cepat**: Semua / Kuat / Belum diunduh.
- **Galeri kartu 5 kolom**, tiap kartu memuat: thumbnail 9:16, nomor urut,
  skor potensi viral (Kuat / Sedang / Biasa), durasi, kutipan pembuka,
  menit asal di video sumber, tombol Unduh dan Putar.

Skor ditandai warna **dan** kata, tidak pernah warna saja.

---

## 7. Token visual

### 7.1 Warna

| Peran              | Hex                                                            |
| ------------------ | -------------------------------------------------------------- |
| Latar halaman      | `#F3F4F7`                                                      |
| Permukaan kartu    | `#FFFFFF`                                                      |
| Garis kartu        | `#E3E6EC`                                                      |
| Garis kontrol      | `#D3D8E0`                                                      |
| Teks utama         | `#15181D`                                                      |
| Teks sekunder      | `#5A6472`                                                      |
| Teks tersier       | `#8A93A1`                                                      |
| Aksen utama        | `#2F5BEA`                                                      |
| Aksen hover        | `#1E3FB0`                                                      |
| Aksen latar lembut | `#F3F6FF` / garis `#C7D3F7` / teks `#35406B`                   |
| Sukses             | `#1C7C4A` · latar `#E8F6EE` · garis `#B9E2CA` · teks `#11512F` |
| Bahaya             | `#B32318` · garis `#F0C4C0`                                    |
| Skor kuat          | `#D64545` (teks putih)                                         |
| Skor sedang        | `#E8A33D` (teks `#15181D`)                                     |

Warna subtitle yang disediakan: `#FFD400` kuning, `#FFFFFF` putih,
`#55E08A` hijau, `#FF8A3D` oranye.

Semua teks memenuhi kontras 4.5:1 terhadap latarnya (3:1 untuk ukuran ≥24px).
Warna yang harus dibedakan juga berbeda terang-gelapnya, bukan hanya rona.

### 7.2 Tipografi

- UI: **Plus Jakarta Sans** — 400, 500, 600, 700
- Log teknis: **JetBrains Mono** — 400

| Peran             | Ukuran / tebal |
| ----------------- | -------------- |
| Judul layar besar | 26px / 700     |
| Nama aplikasi     | 22px / 700     |
| Judul kartu       | 18px / 700     |
| Teks utama tebal  | 16px / 600     |
| Teks kontrol      | 15px / 600     |
| Teks biasa        | 14px / 400     |
| Meta / keterangan | 13px / 400     |
| Badge             | 11px / 700     |

### 7.3 Bentuk & jarak

| Properti                 | Nilai                            |
| ------------------------ | -------------------------------- |
| Sudut kartu              | 16px                             |
| Sudut kontrol / tombol   | 10px                             |
| Sudut kartu pilihan      | 12px                             |
| Sudut badge              | 5–6px                            |
| Pill                     | 999px                            |
| Padding halaman          | 32px atas-bawah, 40px kiri-kanan |
| Padding kartu            | 24px (28px untuk kartu status)   |
| Jarak antar kartu        | 20px                             |
| Jarak antar kolom        | 32px                             |
| Jarak dalam grid pilihan | 12px                             |

### 7.4 Ukuran kontrol

| Kontrol                    | Tinggi  |
| -------------------------- | ------- |
| Tombol header / aksi utama | 48px    |
| Input, select, stepper     | 44px    |
| Tombol kecil dalam kartu   | 38–40px |

Semua target sentuh minimal 44px pada sisi terpendek yang relevan.

---

## 8. Komponen

| Komponen         | Catatan implementasi                                                        |
| ---------------- | --------------------------------------------------------------------------- |
| `Card`           | Permukaan putih, garis 1px, radius 16, padding 24                           |
| `StepCard`       | Card + lingkaran nomor 28px hitam di kiri judul                             |
| `ChoiceCard`     | Tombol; terpilih = garis 2px aksen + ring `rgba(47,91,234,0.12)`            |
| `Segmented`      | Baris tombol dalam satu kotak bergaris; aktif = latar `#15181D`, teks putih |
| `Swatch`         | 44×44, radius 10; terpilih = garis 2px `#15181D` + ring                     |
| `Stepper`        | Tombol −/+ 44×44 mengapit input readonly 72px                               |
| `Preview916`     | `aspect-ratio: 9/16`, overlay area aman, contoh subtitle live               |
| `ProgressBar`    | Tinggi 10px, radius 999, isi aksen                                          |
| `StageCard`      | Tiga status: aktif (biru lembut), menunggu (abu), selesai (centang)         |
| `ClipCard`       | Thumbnail 9:16 + badge + kutipan + aksi                                     |
| `Banner`         | Varian sukses / info; ikon lingkaran 34px di kiri                           |
| `AdvancedDrawer` | `<details>` + `<summary>`, tertutup secara bawaan                           |

Semua kontrol memakai elemen asli: `<button>`, `<a href>`, `<input>` + `<label>`,
`<select>`. Tidak ada `role`/`onClick` pada `div`. Tombol khusus ikon wajib
punya `aria-label`.

---

## 9. State yang harus ada

| Layar     | State                      | Tampilan                                                                |
| --------- | -------------------------- | ----------------------------------------------------------------------- |
| Atur klip | Belum ada video            | Kartu 1 jadi area unggah; kartu 2–4 dan tombol utama nonaktif           |
| Atur klip | Video terlalu pendek       | Peringatan di kartu 4: jumlah klip dibatasi otomatis                    |
| Proses    | Gagal di tahap tertentu    | Kartu tahap jadi merah + penjelasan + tombol "Coba lagi dari tahap ini" |
| Proses    | Dibatalkan                 | Kembali ke layar 1, klip yang sudah jadi tetap tersimpan                |
| Hasil     | Sebagian klip gagal render | Kartu gagal ditandai, tombol "Render ulang yang gagal"                  |
| Hasil     | Nol klip layak             | Banner netral + saran menurunkan ambang / menambah jumlah klip          |

---

## 10. Aturan penulisan teks

- Bahasa Indonesia, kalimat pendek, sapaan netral (hindari "Anda" berulang).
- Tidak memakai istilah mesin di layar utama: _fit_, _pacing_, _outline_,
  _safe area_, _burned-in_, _engine_, _FPS_ hanya boleh muncul di laci lanjutan.
- Angka selalu diberi satuan atau konteks: "42 menit 18 detik", bukan "2538".
- Tombol memakai kata kerja + objek: "Buat 15 klip", "Unduh semua (15)",
  "Ganti video" — bukan "OK", "Submit", "Process".
- Pesan error menyebutkan apa yang terjadi dan apa langkah berikutnya.
- Setiap kartu punya satu kalimat penjelas di bawah judul bila fungsinya tidak
  jelas dari namanya sendiri.

---

## 11. Pemetaan UI lama → UI baru

| Kontrol lama                                                       | Jadi apa                                                              |
| ------------------------------------------------------------------ | --------------------------------------------------------------------- |
| Font, Colour, Size, Style, Pacing, Outline                         | Kartu 2: tiga preset + ukuran + warna. Font & outline → laci lanjutan |
| Position `540·1440`, Platform, Safe area                           | Kartu 3: pilih TikTok / Reels / Shorts                                |
| Fit, Background, Zoom                                              | Dihitung otomatis dari platform; override di laci lanjutan            |
| Watermark                                                          | Laci lanjutan                                                         |
| Whisper model, Engine, Model, Test, Correct with AI, Names & terms | Laci lanjutan                                                         |
| Resolution, Quality, FPS                                           | Laci lanjutan (bawaan 1080p / HD / ikut sumber)                       |
| Clip length, Max clips, Save as                                    | Kartu 4                                                               |
| Running · 24% + Process log                                        | Layar 2 penuh                                                         |
| _(tidak ada)_                                                      | Layar 3 — galeri hasil                                                |

---

## 12. Keputusan yang masih terbuka

1. Apakah skor "potensi viral" dihitung dari model atau dari heuristik
   (durasi, kepadatan kata, jeda)? Label di UI menyesuaikan tingkat keyakinan.
2. Apakah preset platform perlu bisa ditambah sendiri oleh tim.
3. Apakah versi tanpa subtitle dihitung terhadap kuota penyimpanan.
4. Perlu tidaknya halaman riwayat batch sebelumnya.

---

## 13. Lampiran — ringkasan nilai bawaan

```
Gaya subtitle   : Sorot kata
Ukuran huruf    : Sedang (94px @1080×1920)
Warna sorotan   : #FFD400
Platform        : TikTok
Jumlah klip     : 15
Panjang klip    : Otomatis
Tanpa subtitle  : aktif
Resolusi        : 1080p
Kualitas        : HD
FPS             : ikut sumber
Model transkrip : large-v3-turbo
Koreksi AI      : aktif
```
