# DESIGN.md — News Cards

Spesifikasi desain UI untuk tool pengubah artikel berita menjadi kartu
1080×1920 beserta caption postingan.

- **Status:** usulan redesign (v1)
- **Tanggal:** 9 Oktober 2026
- **Target pengguna:** tim non-teknis (admin media sosial)
- **Mockup interaktif:** https://claude.ai/artifact/XosJrTrqCojoxxWsBkMWCe
- **Satu keluarga dengan:** `DESIGN.md` (Video Clips) — token, tipografi, dan
  pola komponennya sama persis supaya dua tool ini terasa satu produk.

---

## 1. Masalah pada UI saat ini

| #   | Masalah                                                                                          | Dampak                                                                    |
| --- | ------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------- |
| 1   | Daftar artikel — langkah pertama — ditaruh di kolom kanan                                        | Alur kerja melawan arah baca; pengguna baru bingung harus mulai dari mana |
| 2   | Panel `PARAGRAPH` (Engine, Model `glm-5.3`, Test, Analyse, Paragraphs) menempati pojok kiri atas | Posisi paling menonjol dipakai setelan yang paling jarang disentuh        |
| 3   | Pratinjau kartu kecil (±210×400) dan kosong                                                      | Output utama tool justru paling sulit dinilai                             |
| 4   | Tujuh isian konten tampil sekaligus, sebagian terisi otomatis                                    | Tidak jelas mana yang perlu diedit manusia, mana hasil otomatis           |
| 5   | `Text on the card` hanya 3 baris, tanpa hitungan karakter                                        | Teks kepanjangan baru ketahuan setelah render                             |
| 6   | Caption kosong tanpa petunjuk; hashtag hanya placeholder                                         | Caption sering ditulis ulang dari nol di luar tool                        |
| 7   | `Image URL` diisi tautan mentah                                                                  | Pengguna harus menyalin URL gambar secara manual                          |
| 8   | Istilah internal: _Analyse_, _Paragraphs_, _From photo_, _Paper_, _Text panel_                   | Perlu pelatihan untuk tiap istilah                                        |
| 9   | Tombol `copy` di tiap baris artikel tanpa keterangan                                             | Tidak jelas yang disalin link, judul, atau isi                            |
| 10  | `Save card` abu-abu di dasar panel                                                               | Tidak jelas file disimpan ke mana dan dalam format apa                    |
| 11  | Tidak ada galeri kartu yang pernah dibuat                                                        | Tidak bisa menelusuri atau memakai ulang hasil kemarin                    |

---

## 2. Prinsip desain

1. **Urutan layar mengikuti urutan kerja.** Pilih artikel → susun kartu →
   simpan dan bagikan. Satu layar satu pekerjaan.
2. **Pratinjau adalah produk.** Kartu ditampilkan besar dan selalu terlihat;
   semua perubahan langsung tampak.
3. **Otomatis dulu, koreksi kemudian.** Ringkasan, caption, hashtag, dan foto
   diisi sistem; pengguna tinggal memperbaiki.
4. **Preset menggantikan parameter.** Empat tampilan jadi, bukan lima dropdown
   yang harus dikombinasikan sendiri.
5. **Istilah manusia.** Tidak ada nama engine, model, atau parameter di layar
   utama.
6. **Kredit sumber tidak bisa lupa.** Nama sumber dan tanggal otomatis menempel
   di kartu dan di akhir caption.

---

## 3. Struktur aplikasi

```
[1] Pilih artikel ──pilih──▶ [2] Susun kartu ──simpan──▶ [3] Simpan & bagikan
      ▲                            ▲                              │
      │                            └────── Ubah kartu ini ────────┤
      └───────────────── Buat kartu lain ──────────────────────────┘
```

Header tetap di ketiga layar. Tombol biru hanya satu per layar.

---

## 4. Layar 1 — Pilih artikel

### 4.1 Tata letak

Satu kolom terpusat, lebar maksimal 1180px.

```
┌─ Header ───────────────────────────────────────────────┐
│ News cards                   [Kartu saya] [Pengaturan] │
└────────────────────────────────────────────────────────┘
  Judul halaman + satu kalimat penjelas
┌─ Bilah cari ───────────────────────────────────────────┐
│ [Cari berita        ] [Cari]  atau  [Tempel link] [Ambil]│
└────────────────────────────────────────────────────────┘
  Chip sumber: Semua · ANTARA · CNN Indonesia · Detik
┌──────────┐ ┌──────────┐ ┌──────────┐
│ kartu    │ │ kartu    │ │ kartu    │   grid 3 kolom
│ artikel  │ │ artikel  │ │ artikel  │
└──────────┘ └──────────┘ └──────────┘
```

### 4.2 Aturan

- **Dua jalur masuk setara**: cari dari sumber terhubung, atau tempel link.
  Keduanya dalam satu bilah, dipisah kata "atau".
- **Hasil sebagai kartu**, bukan baris daftar: thumbnail 150px, badge sumber,
  umur berita relatif ("5 jam lalu", bukan tanggal penuh), judul maksimal
  3 baris.
- **Satu tombol per kartu**: "Pilih artikel ini". Kartu yang sedang dipilih
  berubah jadi "Lanjut buat kartu" dan diberi garis aksen.
- Tombol `copy` lama dihapus — menyalin link bukan tujuan pengguna di layar ini.
- Setelan engine dan model pindah ke **Pengaturan** di header.

---

## 5. Layar 2 — Susun kartu

### 5.1 Tata letak

```
┌─ Header: ← Ganti artikel · judul artikel · [Simpan & lanjut] ─┐
┌─ Kiri (flex 1 1 440px) ──┐ ┌─ Kanan (flex 999 1 520px) ──────┐
│ [9:16] [4:5] [1:1]       │ │ 1. Teks di kartu + hitungan      │
│                          │ │ 2. Foto (dari artikel / unggah)  │
│   PRATINJAU KARTU        │ │ 3. Tampilan (4 preset + rata)    │
│   (maks 432px lebar)     │ │ 4. Caption & hashtag             │
│                          │ │ ▸ Atur huruf dan jarak sendiri   │
│ ukuran asli 1080×1920    │ │                                  │
└──────────────────────────┘ └──────────────────────────────────┘
```

Pratinjau di kiri karena dibaca lebih dulu; kontrol di kanan mengikuti arah
kerja mata setelah melihat hasil.

### 5.2 Panel kontrol

**1 · Teks di kartu**
Textarea 3 baris + tombol "Ringkas otomatis". Di bawahnya penghitung karakter
dengan ambang:

| Rentang | Warna            | Teks                                  |
| ------- | ---------------- | ------------------------------------- |
| ≤ 140   | hijau `#1C7C4A`  | "pas untuk satu kartu"                |
| 141–180 | oranye `#E8A33D` | "mulai padat"                         |
| > 180   | merah `#B32318`  | "terlalu panjang, teks akan mengecil" |

**2 · Foto**
Thumbnail 88×88 dari gambar yang ada di artikel, plus kotak "Unggah".
Field `Image URL` mentah dihapus dari layar utama (tetap ada di Pengaturan
untuk kasus khusus).

**3 · Tampilan**
Empat preset dengan contoh warna, menggantikan kombinasi
`Style` + `Colour` + `Text panel`:

| Preset     | Latar panel | Judul     | Isi       | Badge                      |
| ---------- | ----------- | --------- | --------- | -------------------------- |
| Gelap      | `#15181D`   | `#FFFFFF` | `#C8D1DC` | `#FFD400` / teks `#15181D` |
| Terang     | `#FFFFFF`   | `#15181D` | `#5A6472` | `#15181D` / teks `#FFFFFF` |
| Warna foto | `#3C2A1E`   | `#FFF6EC` | `#DEC9B4` | `#E8A33D` / teks `#2A1C12` |
| Kertas     | `#F2EBDD`   | `#2A2318` | `#5E5444` | `#2A2318` / teks `#F2EBDD` |

Di bawahnya hanya satu kontrol lagi: perataan teks (Rata kiri / Rata tengah).

Rasio pindah ke atas pratinjau sebagai pil, karena menyangkut bentuk kartu,
bukan gaya:

| Rasio       | Tinggi area foto |
| ----------- | ---------------- |
| 9:16 Story  | 46%              |
| 4:5 Feed    | 42%              |
| 1:1 Persegi | 38%              |

**4 · Caption untuk postingan**
Textarea caption + tombol "Tulis otomatis", lalu field hashtag terpisah.
Catatan tetap: kredit sumber otomatis ditambahkan.

**Laci — Atur huruf dan jarak sendiri**
Ukuran judul, tinggi baris, lebar panel, posisi logo, outline, dan padding.

---

## 6. Layar 3 — Simpan & bagikan

- Pratinjau kartu final 300px di kiri.
- Banner hijau: "Kartu tersimpan. 1080 × 1920 px, PNG, 412 KB." — ukuran dan
  format disebut, tidak lagi samar.
- Tiga aksi: **Unduh PNG** (gelap, utama), **Salin caption**, **Unduh versi 4:5**.
- Kotak pratinjau caption persis seperti yang akan tersalin, lengkap dengan
  hashtag dan baris kredit.
- **Galeri kartu** yang sudah dibuat, grid 6 kolom, dengan sumber, umur, dan
  tombol unduh per kartu.

---

## 7. Token visual

Identik dengan Video Clips — lihat `DESIGN.md` bagian 7. Ringkasnya:

### 7.1 Warna

| Peran           | Hex                                                                      |
| --------------- | ------------------------------------------------------------------------ |
| Latar halaman   | `#F3F4F7`                                                                |
| Permukaan kartu | `#FFFFFF`                                                                |
| Garis kartu     | `#E3E6EC`                                                                |
| Garis kontrol   | `#D3D8E0`                                                                |
| Teks utama      | `#15181D`                                                                |
| Teks sekunder   | `#5A6472`                                                                |
| Teks tersier    | `#8A93A1`                                                                |
| Aksen utama     | `#2F5BEA` · hover `#1E3FB0`                                              |
| Aksen lembut    | latar `#EEF2FF` / `#F3F6FF`, garis `#C7D3F7`, teks `#2F3C85` / `#35406B` |
| Sukses          | `#1C7C4A` · latar `#E8F6EE` · garis `#B9E2CA` · teks `#11512F`           |
| Peringatan      | `#E8A33D`                                                                |
| Bahaya          | `#B32318`                                                                |

### 7.2 Tipografi

**Plus Jakarta Sans** 400/500/600/700.

| Peran         | Ukuran / tebal  |
| ------------- | --------------- |
| Judul halaman | 28px / 700      |
| Nama aplikasi | 22px / 700      |
| Judul panel   | 17–18px / 700   |
| Teks kontrol  | 15px / 600      |
| Teks biasa    | 14px / 400      |
| Meta          | 12.5–13px / 400 |

Tipografi **di dalam kartu** berbeda dan punya skalanya sendiri (pada kanvas
1080×1920): judul 72px/700, isi 40px/400, badge 32px/700, kaki 34px/400.
Di mockup nilai ini ditampilkan pada skala 40%.

### 7.3 Bentuk & jarak

| Properti            | Nilai                               |
| ------------------- | ----------------------------------- |
| Sudut panel         | 16px                                |
| Sudut kartu hasil   | 12–14px                             |
| Sudut kontrol       | 9–10px                              |
| Pil / chip          | 999px                               |
| Padding halaman     | 28–32px atas-bawah, 40px kiri-kanan |
| Padding panel       | 22px                                |
| Jarak antar panel   | 18px                                |
| Jarak antar kolom   | 32px                                |
| Tinggi tombol utama | 48px                                |
| Tinggi input        | 44–48px                             |
| Tinggi tombol kecil | 34–38px                             |
| Thumbnail foto      | 88×88px                             |

---

## 8. Komponen

| Komponen         | Catatan                                                                                                    |
| ---------------- | ---------------------------------------------------------------------------------------------------------- |
| `SearchBar`      | Dua input setara dipisah kata "atau", masing-masing punya tombol aksinya                                   |
| `SourceChip`     | Pil filter; aktif = latar `#15181D`, teks putih                                                            |
| `ArticleCard`    | Thumbnail 150px + badge + judul + satu tombol; terpilih = garis 2px aksen + ring                           |
| `CardPreview`    | `aspect-ratio` mengikuti rasio terpilih, maksimal 432px, bayangan `0 10px 30px rgba(21,24,29,0.14)`        |
| `RatioPill`      | Tiga pil di atas pratinjau                                                                                 |
| `ThemeTile`      | Kotak contoh warna 54px + nama; terpilih = garis aksen + ring                                              |
| `PhotoThumb`     | 88×88, terpilih = garis aksen + ring                                                                       |
| `CharCounter`    | Teks berwarna sesuai ambang, selalu disertai kata, bukan warna saja                                        |
| `AiButton`       | Latar `#EEF2FF`, garis `#C7D3F7`, ikon bintang garis — menandai aksi yang hasilnya perlu diperiksa manusia |
| `CaptionPreview` | Blok `white-space: pre-line` berisi caption persis seperti yang disalin                                    |
| `HistoryGrid`    | Grid 6 kolom kartu mini 9:16                                                                               |

Semua kontrol memakai elemen asli (`button`, `a href`, `input`+`label`,
`textarea`, `select`). Tombol khusus ikon wajib `aria-label`.

---

## 9. State yang harus ada

| Layar         | State                      | Tampilan                                                                 |
| ------------- | -------------------------- | ------------------------------------------------------------------------ |
| Pilih artikel | Pencarian nol hasil        | Blok kosong + saran kata kunci lain + ajakan tempel link                 |
| Pilih artikel | Link gagal diambil         | Pesan di bawah input: alasan (paywall / bukan artikel) + opsi isi manual |
| Susun kartu   | Artikel tanpa gambar       | Area foto jadi warna blok preset + ajakan unggah                         |
| Susun kartu   | Teks melebihi 180 karakter | Penghitung merah + peringatan di pratinjau                               |
| Susun kartu   | Ringkasan otomatis gagal   | Textarea tetap berisi paragraf pertama artikel + catatan kecil           |
| Simpan        | Render gagal               | Banner merah + tombol "Coba render ulang"                                |
| Galeri        | Belum ada kartu            | Blok kosong + tombol "Buat kartu pertama"                                |

---

## 10. Aturan penulisan teks

- Bahasa Indonesia, kalimat pendek.
- Tidak memakai istilah mesin di layar utama: _engine_, _model_, _analyse_,
  _paragraphs_, _text panel_, _from photo_ hanya di Pengaturan.
- Waktu berita ditulis relatif ("5 jam lalu"); tanggal penuh hanya di kartu.
- Tombol memakai kata kerja + objek: "Pilih artikel ini", "Unduh PNG",
  "Salin caption" — bukan "OK" atau "Save".
- Aksi berbantuan AI diberi label hasil, bukan nama teknologi:
  "Ringkas otomatis", "Tulis otomatis".
- Setiap panel punya satu kalimat penjelas bila fungsinya tidak jelas dari
  namanya.

---

## 11. Pemetaan UI lama → UI baru

| Elemen lama                             | Jadi apa                                                                    |
| --------------------------------------- | --------------------------------------------------------------------------- |
| Panel `PARAGRAPH` (Engine, Model, Test) | Pengaturan di header                                                        |
| Tombol `Analyse` / `Paragraphs`         | "Ringkas otomatis" di panel teks kartu                                      |
| `Title`                                 | Diambil dari artikel, ditampilkan di header layar 2 (tidak diedit di kartu) |
| `Text on the card`                      | Panel 1 "Teks di kartu" + penghitung karakter                               |
| `Caption` + `Hashtags`                  | Panel 4, dengan tombol tulis otomatis                                       |
| `Source (badge)` + `Date`               | Otomatis dari artikel, tampil di kartu; diedit di Pengaturan                |
| `Image URL`                             | Panel 2 "Foto": pilih dari artikel atau unggah                              |
| `Style`, `Colour`, `Text panel`         | Empat preset tampilan                                                       |
| `Ratio`                                 | Pil di atas pratinjau                                                       |
| `Text alignment`                        | Segmented di panel tampilan                                                 |
| `PHOTO`, `TYPE & SPACING`               | Laci "Atur huruf dan jarak sendiri"                                         |
| `Save card`                             | Layar 3: Unduh PNG + Salin caption + galeri                                 |
| Tombol `copy` per artikel               | Dihapus                                                                     |
| Badge `Rendering...` melayang           | Status di tombol simpan + banner hasil                                      |

---

## 12. Keputusan yang masih terbuka

1. Sumber berita mana saja yang resmi terhubung, dan bagaimana menambah sumber.
2. Apakah tim perlu template bermerek sendiri (logo, warna instansi) di luar
   empat preset.
3. Apakah caption otomatis perlu nada berbeda per platform (Instagram vs X).
4. Berapa lama kartu di galeri disimpan, dan siapa yang boleh menghapus.
5. Apakah perlu mode batch: satu artikel jadi beberapa kartu bersambung.

---

## 13. Lampiran — ringkasan nilai bawaan

```
Rasio            : 9:16
Tampilan         : Gelap
Perataan teks    : Rata kiri
Foto             : gambar pertama dari artikel
Panjang ringkasan: maksimal 180 karakter (ideal ≤140)
Caption          : ditulis otomatis, bisa diedit
Kredit sumber    : selalu ikut di kartu dan caption
Format unduhan   : PNG 1080×1920
```
