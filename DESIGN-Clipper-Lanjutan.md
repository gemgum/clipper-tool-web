# DESIGN.md — Clipper (lanjutan: 5 layar)

Spesifikasi desain untuk lima layar sisa aplikasi Clipper: Tulis artikel,
Watermark, Caption, Riwayat hasil, dan Pengaturan — ditambah keputusan
tingkat produk yang mengikat ketujuh alat jadi satu aplikasi.

- **Status:** usulan redesign (v1)
- **Tanggal:** 9 Oktober 2026
- **Target pengguna:** tim non-teknis
- **Mockup interaktif:** https://claude.ai/artifact/8yXMu1Z8uiKBpqXwB3m7YG
- **Seri yang sama:** `DESIGN.md` (Video Clips), `DESIGN-NewsCards.md` (News Cards)

---

## 1. Temuan tingkat produk

Setelah melihat tujuh layar sekaligus, masalah terbesarnya bukan di masing-masing
layar, melainkan di tingkat aplikasi.

| # | Temuan | Bukti di UI lama |
|---|---|---|
| 1 | **Tidak ada kerangka aplikasi.** Tiap alat berdiri sendiri seperti halaman terpisah | Tidak ada navigasi yang terlihat di satu pun tangkapan layar |
| 2 | **Panel AI Engine diulang tiga kali** dengan isi identik | Ada di Article writer, Captions, dan Requirements |
| 3 | **Log hitam diulang di tiap alat** dan memakan ruang meski kosong | "Nothing yet. The log fills up once a job starts." di tiga layar |
| 4 | **Header `Start processing` / `Cancel` diulang** tanpa menyebut akan memproses apa | Empat layar memakai tombol yang sama persis |
| 5 | **Hasil terpisah dari tempat kerja** | Output history berdiri sendiri; alat lain tidak menautkannya |
| 6 | **Koordinat mentah dipakai sebagai kontrol utama** | `540·700`, `540·960`, `Grid 20` di Watermark; `540·1440` di Video Clips |
| 7 | **Kegagalan tidak menjelaskan diri** | Baris riwayat hanya menulis `0 clips · error` |
| 8 | **Requirements mencampur empat urusan** | Kunci API, biner sistem, unduhan model, dan tutorial aplikasi pihak ketiga dalam satu gulungan panjang |

### Keputusan pengikat

1. **Navigasi antar alat ditunda.** Mockup ini sengaja hanya berisi panelnya.
   Rencananya tetap satu sidebar tetap dengan dua grup — *Buat* (Klip video,
   Kartu berita, Tulis artikel, Watermark, Caption) dan *Kelola* (Riwayat hasil,
   Pengaturan) — tapi bentuknya dibahas terpisah, bukan di dokumen ini.
2. **Mesin AI dicabut dari semua alat** dan jadi satu pengaturan global.
   Statusnya cukup satu baris tipis di atas panel, selalu terlihat.
3. **Log teknis dilipat** (`<details>`), tidak pernah lagi jadi blok hitam tetap.
4. **Tombol utama menyebut pekerjaannya**: "Tulis artikel dari 3 sumber",
   "Proses 3 video" — bukan "Start processing".
5. **Riwayat satu untuk semua alat**, disaring per alat; tiap alat menautkannya.
6. **Kegagalan selalu menyebut sebab dan jalan keluar.**

---

## 2. Kerangka panel

```
┌──────────────────────────────────────────────────────────┐
│ Clipper / Tulis artikel          ● Mesin AI siap · glm-5.3│  ← strip 38px
├──────────────────────────────────────────────────────────┤
│ Judul alat + satu kalimat penjelas        [aksi utama]    │  ← header
├──────────────────────────────────────────────────────────┤
│                                                          │
│                     Area kerja                           │
│                                                          │
├──────────────────────────────────────────────────────────┤
│ ▸ Catatan teknis                                         │  ← log, terlipat
└──────────────────────────────────────────────────────────┘
```

- **Strip identitas** (tinggi 38px, latar putih, garis bawah `#E3E6EC`):
  nama aplikasi + nama alat di kiri, keadaan mesin AI di kanan
  ("Mesin AI siap · glm-5.3", "1 pekerjaan jalan · 24%"). Ini yang menggantikan
  kartu status sidebar sampai navigasi digambar.
- **Header alat**: judul (20px/700), satu kalimat penjelas (13.5px/`#5A6472`),
  aksi utama di kanan — teksnya menyebut pekerjaannya.
- **Area kerja**: `padding: 26px 32px 40px`, panel-panel `flex-wrap: wrap` dengan
  `gap: 26px` sehingga kolom menumpuk sendiri di layar sempit.
- **Catatan teknis**: `<details>` di kaki panel, tertutup kalau pekerjaan sukses.

Semua panel dibuat tanpa `position: fixed` dan tanpa `height: 100vh` di dalamnya,
jadi aman dipakai di lebar berapa pun.

---

## 3. Layar — Tulis artikel

**Janji alat:** gabungkan sampai 5 berita jadi satu artikel, tiap fakta
membawa sumbernya. Di UI lama janji itu tidak terlihat di mana pun.

- **Kiri (400px)** — *Sumber terpilih*: daftar bernomor 1–5, tiap baris bisa
  dikeluarkan. Di bawahnya kolom tempel link, lalu *Berita hari ini* sebagai
  sumber usulan dengan tombol Tambah per baris.
- **Kanan (lebar sisa)** — artikel hasil, ditampilkan sebagai artikel sungguhan:
  judul, paragraf, dan **penanda sumber `¹ ² ³` menempel di akhir tiap kalimat**.
  Penanda memakai gaya chip (`#EEF2FF` / `#2F5BEA`, 11px/700) agar terbaca
  sebagai tautan, bukan catatan kaki akademis.
- Daftar sumber lengkap di kaki artikel.
- Aksi: Salin teks, Unduh .docx.
- Panel AI Engine dan log hitam dihapus dari layar ini.

**Aturan:** keranjang maksimal 5; tombol Tambah mati saat penuh dan
menjelaskan sebabnya.

---

## 4. Layar — Watermark

Perubahan terbesar: **koordinat diganti tarikan langsung**.

| Lama | Baru |
|---|---|
| `Position 540·700` | Seret logo di pratinjau + sembilan titik jangkar sebagai jalan cepat |
| `Headline position 540·960` | Seret judul di pratinjau |
| `Grid 20` | Dihapus — tidak perlu saat posisi bisa diseret |
| `Width 25%` + `Height 25%` | Satu kontrol "Ukuran logo" (logo dijaga proporsional) |
| `Appears 0s` + `Stays 0s` | Dua pilihan kalimat: "Tampil sepanjang video" atau "Muncul di detik ke-N selama M detik" |
| `Headline size 64` | Kecil / Sedang / Besar |
| `Colour` + `Outline 3` | Tiga warna; garis tepi dihitung otomatis dari warna dan ukuran |
| Panel berjudul `WATERMARK · OFF` tapi isinya aktif | Sakelar "Pakai logo" yang jelas |

**Sembilan titik jangkar** (grid 3×3, tiap sel 40px) memetakan ke posisi:

| | Kiri | Tengah | Kanan |
|---|---|---|---|
| **Atas** | 5%, 4% | 50%, 4% | 5% dari kanan, 4% |
| **Tengah** | 5%, 46% | 50%, 46% | 5% dari kanan, 46% |
| **Bawah** | 5%, 16% dari bawah | 50%, 16% dari bawah | 5% dari kanan, 16% dari bawah |

**Daftar video** kini menampilkan status per berkas. Video non-9:16 ditandai
"bukan 9:16" dan dilewati — tidak menggagalkan seluruh antrean. Kalimat
"refused by name" dihapus karena menolak berdasarkan nama berkas itu rapuh;
rasio dibaca dari berkasnya.

---

## 5. Layar — Caption

**Masalah lama:** area hasil kosong tanpa contoh bentuk keluaran, dan dua
angka yang maknanya tidak jelas (`Listen to 5 min`, `Per video 3`).

- **Hasil jadi isi utama**: satu kartu per video, berisi **tiga pilihan caption**
  dengan label gaya (LANGSUNG / BERTANYA / RINGKAS), jumlah karakter, dan
  tombol Salin per pilihan.
- Tombol "Lihat transkrip" per video untuk memeriksa sumber kalimatnya.
- **Setelan diberi kalimat:**
  - `Listen to 5 min` → "Dengarkan berapa lama" dengan pilihan 2 menit /
    5 menit / Semua, plus penjelasan kenapa menit awal biasanya cukup.
  - `Per video 3` → "Berapa pilihan caption per video".
  - `Names & terms` dipertahankan — ini berguna dan sudah jelas; hanya diberi
    kalimat penjelas.
- Panel AI Engine dan Whisper model pindah ke Pengaturan.

---

## 6. Layar — Riwayat hasil

- **Satu riwayat untuk kelima alat.** Saringan berupa chip: Semua, Klip video,
  Kartu berita, Artikel, Caption, Gagal — masing-masing membawa angka.
- **Pekerjaan berjalan** tampil sebagai kartu bergaris aksen di paling atas:
  nama berkas, tahap, progress bar, perkiraan sisa waktu, tombol Batalkan.
- **Pekerjaan selesai**: badge alat, nama sumber, waktu, jumlah hasil,
  "Pilih semua", "Unduh semua", lalu kartu hasil.
- **Kartu klip** mempertahankan skor, tapi singkatan `hook · emo · clear` yang
  terpotong diganti **baris keterangan di bawah daftar**:
  - *Arti skor* — seberapa layak klip berdiri sendiri sebagai konten
  - *Pembuka* — kekuatan 10 detik pertama
  - *Emosi* — muatan rasa dalam ucapan
  - *Jelas* — mudah dipahami tanpa konteks
- **Pekerjaan gagal** tidak lagi menulis `0 clips · error`, melainkan:
  > Berhenti di tahap transkrip: model ggml-large-v3 belum diunduh.
  dengan dua tombol: "Buka Pengaturan" dan "Ulangi".

---

## 7. Layar — Pengaturan (dulu: Requirements)

Gulungan panjang dipecah jadi tiga tab.

**Tab 1 · Mesin AI**
- Kartu "Mesin yang dipakai sekarang" di atas: nama, model, alamat, tombol
  Uji koneksi dan Ganti.
- Daftar mesin lain dengan status berkata-kata: "belum ada kunci",
  "tidak jalan" — bukan hanya titik warna.
- Kunci API tidak pernah ditampilkan ulang; hanya "Isi kunci" atau "Ganti kunci".

**Tab 2 · Program & model**
- Program sistem (ffmpeg, ffprobe, whisper-cli) dengan keterangan fungsi.
- **Model pengenalan suara dengan tombol Unduh / Hapus per baris** — ini yang
  paling hilang di UI lama: daftar enam model ditampilkan, tapi tidak ada cara
  mengunduh yang belum ada. Ditambah ringkasan "2 dari 6 terpasang · 3,0 GB terpakai".
- Panduan AI lokal (Ollama, LM Studio, Jan, llama.cpp, KoboldCpp, GPT4All)
  dilipat — itu petunjuk sekali pakai, bukan setelan harian.

**Tab 3 · Lokasi berkas**
Daftar path penyimpanan dalam huruf monospace.

**Panel kanan · Kesiapan sistem**
Ringkasan empat baris yang selalu terlihat, dengan catatan: kalau satu baris
merah, alat yang membutuhkannya akan menolak jalan dan menyebut baris ini
sebagai alasannya.

---

## 8. Token visual

Sama persis dengan dua dokumen sebelumnya.

| Peran | Hex |
|---|---|
| Latar halaman | `#F3F4F7` |
| Permukaan | `#FFFFFF` · garis `#E3E6EC` · garis kontrol `#D3D8E0` |
| Teks | utama `#15181D` · sekunder `#5A6472` · tersier `#8A93A1` |
| Aksen | `#2F5BEA` · lembut `#EEF2FF` / `#F3F6FF` · garis `#C7D3F7` · teks `#2F3C85` |
| Sukses | `#1C7C4A` · latar `#E8F6EE` · teks `#11512F` · titik `#3FBF72` |
| Peringatan | `#E8A33D` |
| Bahaya | `#B32318` · latar `#FDECEA` · garis `#F0C4C0` · teks `#7A2018` |

Tipografi: **Plus Jakarta Sans** (UI), **JetBrains Mono** (path & log).
Judul alat 20px/700 · judul panel 17px/700 · kontrol 14.5px/600 ·
isi 14px/400 · meta 12.5–13px/400.

Bentuk: panel radius 16 · kontrol radius 9–10 · kartu hasil radius 12 ·
chip 999 · padding panel 22 · jarak antar panel 18 · padding area kerja 26/32.

Ukuran: tombol utama 44 · input 42–44 · tombol kecil 32–38 ·
titik status 8–10 · jangkar posisi 40.

---

## 9. Komponen baru di seri ini

| Komponen | Catatan |
|---|---|
| `AppBar` | Strip 38px: nama alat + keadaan mesin AI (pengganti sementara kartu status sidebar) |
| `StatusDot` | Titik 8–10px selalu berpasangan dengan kata, tidak pernah sendirian |
| `ToolBadge` | Badge alat di riwayat (KLIP VIDEO / KARTU BERITA / GAGAL) |
| `SourceChip` | Penanda sumber `¹` dalam teks artikel |
| `AnchorGrid` | Grid 3×3 penentu posisi watermark |
| `DraggableOverlay` | Logo dan judul yang bisa diseret di pratinjau |
| `CaptionOption` | Kartu satu pilihan caption: label gaya + panjang + Salin |
| `JobCard` | Baris riwayat: berjalan / selesai / gagal, masing-masing beda isi |
| `ModelRow` | Baris model dengan ukuran, status, dan tombol Unduh/Hapus |
| `SettingsTabs` | Tiga tab dengan garis bawah aksen 3px |
| `FailureNotice` | Kartu gagal: sebab + dua jalan keluar |

---

## 10. State yang harus ada

| Layar | State | Tampilan |
|---|---|---|
| Tulis artikel | Keranjang kosong | Kotak putus-putus + ajakan ambil dari daftar kanan; tombol utama mati |
| Tulis artikel | Keranjang penuh (5) | Tombol Tambah mati + keterangan "maksimal 5 sumber" |
| Tulis artikel | Satu sumber gagal diambil | Baris itu ditandai, artikel tetap bisa ditulis dari sisanya |
| Watermark | Belum ada logo | Pratinjau hanya menampilkan judul; kotak logo jadi area unggah |
| Watermark | Ada video non-9:16 | Baris ditandai "bukan 9:16", dilewati, bukan menggagalkan antrean |
| Caption | Video tanpa suara | Kartu hasil berisi catatan "tidak ada ucapan terdeteksi" |
| Riwayat | Belum ada pekerjaan | Blok kosong + tautan ke lima alat |
| Riwayat | Pekerjaan gagal | Sebab + tombol Ulangi + tautan ke halaman yang bisa memperbaikinya |
| Pengaturan | Model sedang diunduh | Baris model menampilkan progres dan tombol Batalkan |
| Pengaturan | Uji koneksi gagal | Pesan di bawah kartu mesin: kode kesalahan + saran |

---

## 11. Pemetaan UI lama → UI baru

| Elemen lama | Jadi apa |
|---|---|
| Header `Start processing` / `Cancel` di tiap alat | Tombol utama yang menyebut pekerjaannya; Batalkan hanya muncul saat berjalan |
| Panel `AI ENGINE` (3 tempat) | Pengaturan → tab Mesin AI; status di strip atas |
| `PROCESS LOG` hitam (4 tempat) | `<details>` "Catatan teknis", tertutup |
| `BASKET 0/5` di kanan | Kiri, sebagai daftar bernomor yang bisa diurutkan |
| `Use a different engine to write` | Pengecualian per alat di Pengaturan |
| `Position` / `Headline position` / `Grid` | Seret di pratinjau + 9 titik jangkar |
| `Appears` / `Stays` | Dua pilihan kalimat |
| `Width` + `Height` watermark | Satu kontrol ukuran |
| `Listen to` / `Per video` | "Dengarkan berapa lama" / "Berapa pilihan caption per video" |
| Tab `Results` / `News cards` | Chip saringan per alat, satu daftar |
| `0 clips · error` | Kartu gagal dengan sebab dan dua tombol |
| `hook 80 · emo 80 · clear 80 ·…` | Baris keterangan arti skor di bawah daftar |
| `Requirements` satu gulungan | Tiga tab + panel kesiapan sistem |
| Daftar model tanpa aksi | Tombol Unduh / Hapus per model + ringkasan ruang terpakai |
| `Where things are kept` | Tab Lokasi berkas |

---

## 12. Keputusan yang masih terbuka

1. Apakah tiap alat boleh memakai mesin AI berbeda, atau satu untuk semua.
2. Berapa lama hasil disimpan sebelum dibersihkan otomatis, dan siapa yang
   boleh menghapus.
3. Apakah perlu antrean: menumpuk beberapa pekerjaan dan menjalankannya berurutan.
4. Apakah watermark perlu disimpan sebagai preset bermerek (logo + posisi +
   gaya judul) yang bisa dipakai ulang.
5. Apakah skor klip perlu bisa disetel ambangnya oleh tim.
