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

| #   | Temuan                                                                              | Bukti di UI lama                                                                                       |
| --- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| 1   | **Tidak ada kerangka aplikasi.** Tiap alat berdiri sendiri seperti halaman terpisah | Tidak ada navigasi yang terlihat di satu pun tangkapan layar                                           |
| 2   | **Panel AI Engine diulang tiga kali** dengan isi identik                            | Ada di Article writer, Captions, dan Requirements                                                      |
| 3   | **Log hitam diulang di tiap alat** dan memakan ruang meski kosong                   | "Nothing yet. The log fills up once a job starts." di tiga layar                                       |
| 4   | **Header `Start processing` / `Cancel` diulang** tanpa menyebut akan memproses apa  | Empat layar memakai tombol yang sama persis                                                            |
| 5   | **Hasil terpisah dari tempat kerja**                                                | Output history berdiri sendiri; alat lain tidak menautkannya                                           |
| 6   | **Koordinat mentah dipakai sebagai kontrol utama**                                  | `540·700`, `540·960`, `Grid 20` di Watermark; `540·1440` di Video Clips                                |
| 7   | **Kegagalan tidak menjelaskan diri**                                                | Baris riwayat hanya menulis `0 clips · error`                                                          |
| 8   | **Requirements mencampur empat urusan**                                             | Kunci API, biner sistem, unduhan model, dan tutorial aplikasi pihak ketiga dalam satu gulungan panjang |

### Keputusan pengikat

1. **Navigasi dipisah ke dokumennya sendiri** — lihat `DESIGN-Navigasi.md`.
   Panel-panel di dokumen ini sengaja digambar tanpa sidebar; keduanya bertemu
   dengan sidebar 220px di kiri dan panel mengisi sisanya.
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
  nama aplikasi + nama alat di kiri, keadaan mesin AI di kanan. Begitu sidebar
  dari `DESIGN-Navigasi.md` dipasang, strip ini menyusut jadi breadcrumb saja —
  status mesin AI pindah ke tombol status di kaki sidebar.
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

- **Kiri (400px)** — _Sumber terpilih_: daftar bernomor 1–5, tiap baris bisa
  dikeluarkan. Di bawahnya kolom tempel link, lalu _Berita hari ini_ sebagai
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

| Lama                                               | Baru                                                                                     |
| -------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `Position 540·700`                                 | Seret logo di pratinjau + sembilan titik jangkar sebagai jalan cepat                     |
| `Headline position 540·960`                        | Seret judul di pratinjau                                                                 |
| `Grid 20`                                          | Dihapus — tidak perlu saat posisi bisa diseret                                           |
| `Width 25%` + `Height 25%`                         | Satu kontrol "Ukuran logo" (logo dijaga proporsional)                                    |
| `Appears 0s` + `Stays 0s`                          | Dua pilihan kalimat: "Tampil sepanjang video" atau "Muncul di detik ke-N selama M detik" |
| `Headline size 64`                                 | Kecil / Sedang / Besar                                                                   |
| `Colour` + `Outline 3`                             | Tiga warna; garis tepi dihitung otomatis dari warna dan ukuran                           |
| Panel berjudul `WATERMARK · OFF` tapi isinya aktif | Sakelar "Pakai logo" yang jelas                                                          |

**Sembilan titik jangkar** (grid 3×3, tiap sel 40px) memetakan ke posisi:

|            | Kiri               | Tengah              | Kanan                         |
| ---------- | ------------------ | ------------------- | ----------------------------- |
| **Atas**   | 5%, 4%             | 50%, 4%             | 5% dari kanan, 4%             |
| **Tengah** | 5%, 46%            | 50%, 46%            | 5% dari kanan, 46%            |
| **Bawah**  | 5%, 16% dari bawah | 50%, 16% dari bawah | 5% dari kanan, 16% dari bawah |

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
  - _Arti skor_ — seberapa layak klip berdiri sendiri sebagai konten
  - _Pembuka_ — kekuatan 10 detik pertama
  - _Emosi_ — muatan rasa dalam ucapan
  - _Jelas_ — mudah dipahami tanpa konteks
- **Pekerjaan gagal** tidak lagi menulis `0 clips · error`, melainkan:
  > Berhenti di tahap transkrip: model ggml-large-v3 belum diunduh.
  > dengan dua tombol: "Buka Pengaturan" dan "Ulangi".

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

| Peran         | Hex                                                                         |
| ------------- | --------------------------------------------------------------------------- |
| Latar halaman | `#F3F4F7`                                                                   |
| Permukaan     | `#FFFFFF` · garis `#E3E6EC` · garis kontrol `#D3D8E0`                       |
| Teks          | utama `#15181D` · sekunder `#5A6472` · tersier `#8A93A1`                    |
| Aksen         | `#2F5BEA` · lembut `#EEF2FF` / `#F3F6FF` · garis `#C7D3F7` · teks `#2F3C85` |
| Sukses        | `#1C7C4A` · latar `#E8F6EE` · teks `#11512F` · titik `#3FBF72`              |
| Peringatan    | `#E8A33D`                                                                   |
| Bahaya        | `#B32318` · latar `#FDECEA` · garis `#F0C4C0` · teks `#7A2018`              |

Tipografi: **Plus Jakarta Sans** (UI), **JetBrains Mono** (path & log).
Judul alat 20px/700 · judul panel 17px/700 · kontrol 14.5px/600 ·
isi 14px/400 · meta 12.5–13px/400.

Bentuk: panel radius 16 · kontrol radius 9–10 · kartu hasil radius 12 ·
chip 999 · padding panel 22 · jarak antar panel 18 · padding area kerja 26/32.

Ukuran: tombol utama 44 · input 42–44 · tombol kecil 32–38 ·
titik status 8–10 · jangkar posisi 40.

---

## 9. Komponen baru di seri ini

| Komponen           | Catatan                                                                             |
| ------------------ | ----------------------------------------------------------------------------------- |
| `AppBar`           | Strip 38px: nama alat + keadaan mesin AI (pengganti sementara kartu status sidebar) |
| `StatusDot`        | Titik 8–10px selalu berpasangan dengan kata, tidak pernah sendirian                 |
| `ToolBadge`        | Badge alat di riwayat (KLIP VIDEO / KARTU BERITA / GAGAL)                           |
| `SourceChip`       | Penanda sumber `¹` dalam teks artikel                                               |
| `AnchorGrid`       | Grid 3×3 penentu posisi watermark                                                   |
| `DraggableOverlay` | Logo dan judul yang bisa diseret di pratinjau                                       |
| `CaptionOption`    | Kartu satu pilihan caption: label gaya + panjang + Salin                            |
| `JobCard`          | Baris riwayat: berjalan / selesai / gagal, masing-masing beda isi                   |
| `ModelRow`         | Baris model dengan ukuran, status, dan tombol Unduh/Hapus                           |
| `SettingsTabs`     | Tiga tab dengan garis bawah aksen 3px                                               |
| `FailureNotice`    | Kartu gagal: sebab + dua jalan keluar                                               |

---

## 10. State yang harus ada

| Layar         | State                     | Tampilan                                                              |
| ------------- | ------------------------- | --------------------------------------------------------------------- |
| Tulis artikel | Keranjang kosong          | Kotak putus-putus + ajakan ambil dari daftar kanan; tombol utama mati |
| Tulis artikel | Keranjang penuh (5)       | Tombol Tambah mati + keterangan "maksimal 5 sumber"                   |
| Tulis artikel | Satu sumber gagal diambil | Baris itu ditandai, artikel tetap bisa ditulis dari sisanya           |
| Watermark     | Belum ada logo            | Pratinjau hanya menampilkan judul; kotak logo jadi area unggah        |
| Watermark     | Ada video non-9:16        | Baris ditandai "bukan 9:16", dilewati, bukan menggagalkan antrean     |
| Caption       | Video tanpa suara         | Kartu hasil berisi catatan "tidak ada ucapan terdeteksi"              |
| Riwayat       | Belum ada pekerjaan       | Blok kosong + tautan ke lima alat                                     |
| Riwayat       | Pekerjaan gagal           | Sebab + tombol Ulangi + tautan ke halaman yang bisa memperbaikinya    |
| Pengaturan    | Model sedang diunduh      | Baris model menampilkan progres dan tombol Batalkan                   |
| Pengaturan    | Uji koneksi gagal         | Pesan di bawah kartu mesin: kode kesalahan + saran                    |

---

## 11. Pemetaan UI lama → UI baru

| Elemen lama                                       | Jadi apa                                                                     |
| ------------------------------------------------- | ---------------------------------------------------------------------------- |
| Header `Start processing` / `Cancel` di tiap alat | Tombol utama yang menyebut pekerjaannya; Batalkan hanya muncul saat berjalan |
| Panel `AI ENGINE` (3 tempat)                      | Pengaturan → tab Mesin AI; status di strip atas                              |
| `PROCESS LOG` hitam (4 tempat)                    | `<details>` "Catatan teknis", tertutup                                       |
| `BASKET 0/5` di kanan                             | Kiri, sebagai daftar bernomor yang bisa diurutkan                            |
| `Use a different engine to write`                 | Pengecualian per alat di Pengaturan                                          |
| `Position` / `Headline position` / `Grid`         | Seret di pratinjau + 9 titik jangkar                                         |
| `Appears` / `Stays`                               | Dua pilihan kalimat                                                          |
| `Width` + `Height` watermark                      | Satu kontrol ukuran                                                          |
| `Listen to` / `Per video`                         | "Dengarkan berapa lama" / "Berapa pilihan caption per video"                 |
| Tab `Results` / `News cards`                      | Chip saringan per alat, satu daftar                                          |
| `0 clips · error`                                 | Kartu gagal dengan sebab dan dua tombol                                      |
| `hook 80 · emo 80 · clear 80 ·…`                  | Baris keterangan arti skor di bawah daftar                                   |
| `Requirements` satu gulungan                      | Tiga tab + panel kesiapan sistem                                             |
| Daftar model tanpa aksi                           | Tombol Unduh / Hapus per model + ringkasan ruang terpakai                    |
| `Where things are kept`                           | Tab Lokasi berkas                                                            |

---

## 12. Keputusan yang masih terbuka

1. Apakah tiap alat boleh memakai mesin AI berbeda, atau satu untuk semua.
2. Berapa lama hasil disimpan sebelum dibersihkan otomatis, dan siapa yang
   boleh menghapus.
3. Apakah perlu antrean: menumpuk beberapa pekerjaan dan menjalankannya berurutan.
4. Apakah watermark perlu disimpan sebagai preset bermerek (logo + posisi +
   gaya judul) yang bisa dipakai ulang.
5. Apakah skor klip perlu bisa disetel ambangnya oleh tim.

# Clipper — Navigasi (sidebar)

Dokumen ini khusus membahas satu komponen: rail navigasi di sisi kiri aplikasi
Clipper. Panel-panel alatnya dibahas di `DESIGN.md`, `DESIGN-NewsCards.md`, dan
`DESIGN-Clipper-Lanjutan.md`.

Mockup: artboard **Navigasi (tinjauan)** pada kanvas
_Clipper — Redesign Lanjutan_.

---

## 1. Keadaan sekarang

Rail gelap selebar ±90px, tinggi penuh. Isinya dari atas ke bawah: wordmark
"Clipper", enam item menu (Video clips, News cards, Article writer, Captions,
Watermark, Output history), ruang kosong besar, lalu titik hijau + "AI ready" +
`glm-5.3`, dan tiga ikon tanpa label (akun, tema, gerigi).

### Temuan

| #   | Temuan                                                                           | Akibatnya                                                       |
| --- | -------------------------------------------------------------------------------- | --------------------------------------------------------------- |
| 1   | **Lebar 90px** — terlalu sempit untuk teks, terlalu lebar untuk ikon saja        | Terjebak di antara dua pola; tidak mendapat keuntungan keduanya |
| 2   | **Dua perlakuan kotak** — "Video clips" berkotak abu, "News cards" berkotak biru | Pengguna ragu sedang berada di mana                             |
| 3   | **Status aktif memakai tiga sinyal** — isi biru + garis tepi biru + teks biru    | Berlebihan, dan biru pekat di atas hitam bergetar               |
| 4   | **Label abu ±11px di atas hitam**                                                | Di bawah ambang baca yang nyaman                                |
| 5   | **Enam item satu tumpukan tanpa kelompok**                                       | "Output history" disamakan dengan alat, padahal bukan alat      |
| 6   | **Label panjang membungkus 2–3 baris**                                           | Tinggi baris tidak rata, daftar jadi sulit dipindai             |
| 7   | **Separuh tinggi rail kosong**                                                   | Ruang paling berharga di layar terbuang                         |
| 8   | **`glm-5.3` ditampilkan mentah**                                                 | Tidak berarti bagi pengguna non-teknis                          |
| 9   | **Tiga ikon bawah tanpa label dan sederajat**                                    | Akun, tema, dan pengaturan beda bobot tapi terlihat sama        |
| 10  | **Tidak ada tanda pekerjaan berjalan**                                           | Padahal inti aplikasi ini memproses video yang makan menit      |

---

## 2. Keputusan

1. **Pilih satu pola lebar.** Entah rail ikon murni 72px, atau sidebar berlabel
   220px. Tidak ada di antaranya.
2. **Satu perlakuan aktif:** batang kiri 3px `#2F5BEA` + latar `#1F2430` + teks
   putih. Tidak ada garis tepi pada item menu, dalam keadaan apa pun.
3. **Dua kelompok:** _BUAT_ (alat) dan _KELOLA_ (hasil + setelan).
4. **Label diperpendek** sampai muat satu baris.
5. **Pekerjaan berjalan terlihat dari navigasi**, selalu.
6. **Nama model tidak pernah berdiri sendiri** — selalu di bawah kalimat
   berbahasa manusia.

---

## 3. Varian A — rail ikon 72px

Untuk layar sempit, atau bila panel kerja butuh lebar maksimal.

```
┌────┐
│ C  │  wordmark jadi lencana 26px
├────┤
│ ✂  │  44×44 target sentuh, ikon 21px
│▌📄 │  aktif: batang 3px + #1F2430
│ ✎  │
│ cc │
│ ©  │
│────│  garis #2C3340, lebar 28px
│ ↺² │  badge = pekerjaan berjalan
│    │
│ ⬤AI│  36–44px, titik + huruf
│ ⚙  │
│ N  │  avatar 30px
└────┘
```

- Lebar tetap 72px, `padding: 14px 10px`, isi rata tengah.
- Tiap item 44×44 dengan `border-radius: 10px`.
- **Tooltip wajib.** Muncul di kanan item setelah ±400ms, latar `#15181D`,
  teks putih 12.5px, dengan ekor segitiga. Tanpa tooltip, rail ini jadi tebakan.
- `aria-label` pada setiap tombol, bukan hanya tooltip visual.
- Badge angka di Riwayat: minimal 17px, latar `#2F5BEA`, teks putih 11px/700,
  di sudut kanan atas ikon.

---

## 4. Varian B — sidebar berlabel 220px (disarankan)

```
┌──────────────────────┐
│ [C] Clipper          │
│                      │
│ BUAT                 │
│  ✂  Klip video       │
│ ▌📄 Kartu berita     │  ← aktif
│  ✎  Artikel          │
│  cc Caption          │
│  ©  Watermark        │
│                      │
│ KELOLA               │
│  ↺  Riwayat      (2) │
│  ⚙  Pengaturan       │
│                      │
│        ⋮             │
│ ┌──────────────────┐ │
│ │ ● Memproses 2 vid│ │  ← kartu pekerjaan
│ │ ▓▓▓▓▓░░░░░  42%  │ │
│ └──────────────────┘ │
│ ┌──────────────────┐ │
│ │ ● Mesin AI siap ›│ │  ← tombol status
│ │   Custom·glm-5.3 │ │
│ └──────────────────┘ │
│ (N) Nold          ☀ │
└──────────────────────┘
```

### Ukuran

| Bagian           | Nilai                                                 |
| ---------------- | ----------------------------------------------------- |
| Lebar            | 220px tetap (`flex: 0 1 220px; min-width: 204px`)     |
| Padding rail     | `18px 10px`                                           |
| Tinggi item      | 42px, `border-radius: 9px`, `padding: 0 12px`         |
| Jarak ikon–teks  | 11px                                                  |
| Ikon             | 19px, `stroke-width: 1.7`                             |
| Teks item        | 14px / 500 (aktif: 600)                               |
| Label grup       | 11px / 700, `letter-spacing: 1.2px`, margin kiri 12px |
| Jarak antar grup | 18px                                                  |

### Penamaan

| Lama           | Baru             | Alasan                                              |
| -------------- | ---------------- | --------------------------------------------------- |
| Video clips    | **Klip video**   | Muat satu baris                                     |
| News cards     | **Kartu berita** | —                                                   |
| Article writer | **Artikel**      | "writer" tidak menambah arti                        |
| Captions       | **Caption**      | —                                                   |
| Watermark      | **Watermark**    | Sudah dikenal apa adanya                            |
| Output history | **Riwayat**      | "output" mubazir; ini satu riwayat untuk semua alat |
| —              | **Pengaturan**   | Gerigi anonim naik jadi item menu bernama           |

### Kartu pekerjaan berjalan

Mengisi ruang kosong yang dulu menganggur, dan menjawab pertanyaan yang paling
sering muncul di aplikasi pemrosesan video: _ini lagi ngapain_.

- Muncul hanya bila ada pekerjaan berjalan; kalau kosong, rail cukup merapat.
- Isi: titik biru berdenyut + "Memproses 2 video", bar progres 5px, lalu
  "Klip video · 42% · sisa ±4 menit".
- Diklik → membuka Riwayat tersaring ke pekerjaan itu.
- Lebih dari satu pekerjaan: tampilkan yang paling cepat selesai, dengan
  "+1 lagi" di baris keterangan.

### Tombol status mesin AI

Satu baris, bisa diklik, menuju Pengaturan → Mesin AI.

| Keadaan      | Titik     | Baris 1               | Baris 2                     |
| ------------ | --------- | --------------------- | --------------------------- |
| Siap         | `#3FBF72` | Mesin AI siap         | Custom · glm-5.3            |
| Mengunduh    | `#E8A33D` | Mengunduh model       | whisper-large · 62%         |
| Belum diatur | `#6E7785` | Mesin AI belum diatur | Pilih penyedia dulu         |
| Gagal        | `#F0613F` | Kunci API ditolak     | Perbarui kunci untuk lanjut |

Keadaan gagal memakai latar `#2A1714` dan garis `#5A2A22` supaya terlihat tanpa
harus berteriak.

### Baris akun

Avatar 28px + nama + ikon tema. Tema turun dari deretan ikon anonim ke sini
karena ia preferensi pribadi, sederajat dengan akun — bukan sederajat dengan
navigasi.

---

## 5. Keadaan satu baris menu

Semua bertinggi 42px. Yang berubah hanya latar, warna teks, dan batang kiri.

| Keadaan           | Latar         | Teks           | Ikon      | Batang kiri                                 |
| ----------------- | ------------- | -------------- | --------- | ------------------------------------------- |
| Diam              | transparan    | `#B6BEC9`      | `#B6BEC9` | —                                           |
| Hover             | `#1A1E26`     | `#FFFFFF`      | `#FFFFFF` | —                                           |
| Aktif             | `#1F2430`     | `#FFFFFF` 600  | `#5B86FF` | 3px `#2F5BEA`                               |
| Fokus papan ketik | seperti hover | —              | —         | `outline: 2px #5B86FF; outline-offset: 2px` |
| Nonaktif          | transparan    | `opacity: .42` | —         | —                                           |

Keadaan nonaktif dipakai bila alat butuh model yang belum terpasang; keterangan
alasannya ditaruh di kanan baris ("model belum ada").

---

## 6. Token warna navigasi

| Peran                  | Hex                                           |
| ---------------------- | --------------------------------------------- |
| Latar rail             | `#15181D`                                     |
| Permukaan item (aktif) | `#1F2430`                                     |
| Permukaan item (hover) | `#1A1E26`                                     |
| Garis pemisah          | `#2C3340`                                     |
| Teks item diam         | `#B6BEC9`                                     |
| Teks item aktif        | `#FFFFFF`                                     |
| Label grup             | `#6E7785`                                     |
| Keterangan             | `#8A93A1`                                     |
| Aksen aktif            | `#2F5BEA` · ikon aktif `#5B86FF`              |
| Status siap            | `#3FBF72`                                     |
| Status sibuk           | `#E8A33D`                                     |
| Status gagal           | `#F0613F` · latar `#2A1714` · garis `#5A2A22` |
| Avatar                 | `#3A4455`                                     |

---

## 7. Perilaku responsif

| Lebar jendela | Navigasi                                                                          |
| ------------- | --------------------------------------------------------------------------------- |
| ≥ 1100px      | Varian B, 220px                                                                   |
| 760–1099px    | Varian A, 72px, otomatis                                                          |
| < 760px       | Rail menumpuk jadi bar horizontal di atas panel; label disembunyikan, badge tetap |

Rail ikut mengalir sebagai `flex item`, bukan `position: fixed`, dan tidak pernah
memakai `height: 100vh` — supaya aman dipakai di dalam jendela berapa pun,
termasuk saat panel diperbesar.

---

## 8. Aksesibilitas

- `<nav aria-label="Navigasi utama">` membungkus seluruh rail.
- Item aktif memakai `aria-current="page"`, bukan sekadar beda warna.
- Varian A: setiap tombol punya `aria-label`; tooltip visual bukan pengganti.
- Kontras teks diam `#B6BEC9` di atas `#15181D` ≈ 9:1 — jauh di atas syarat.
- Jangan pernah menyampaikan keadaan hanya lewat warna titik: tiap titik status
  selalu berdampingan dengan kalimatnya.
- Urutan fokus papan ketik mengikuti urutan visual; `Esc` menutup tooltip.

---

## 9. Pemetaan lama → baru

| Lama                           | Baru                                                                |
| ------------------------------ | ------------------------------------------------------------------- |
| Rail 90px label membungkus     | 220px label satu baris (atau 72px ikon saja)                        |
| Kotak abu pada item pertama    | Dihapus — hanya item aktif yang berlatar                            |
| Kotak biru + garis + teks biru | Batang kiri 3px + latar `#1F2430` + teks putih                      |
| Enam item satu tumpukan        | Dua kelompok: BUAT dan KELOLA                                       |
| "Output history"               | "Riwayat", pindah ke KELOLA                                         |
| Ruang kosong di tengah         | Kartu pekerjaan berjalan                                            |
| Titik hijau + `glm-5.3` lepas  | Tombol status dua baris, bisa diklik                                |
| Tiga ikon anonim               | Pengaturan naik ke menu; tema turun ke baris akun; akun jadi avatar |
| Tidak ada tanda pekerjaan      | Badge angka di Riwayat + kartu progres                              |

---

## 10. Yang masih perlu diputuskan

1. Apakah rail bisa dilipat manual, atau hanya ikut lebar jendela?
2. Pekerjaan berjalan lebih dari tiga — kartu jadi daftar pendek, atau tetap satu
   dengan "+n lagi"?
3. Alat baru akan ditambah berapa banyak? Di atas sembilan item, kelompok BUAT
   perlu bisa digulung.
4. Apakah tema terang benar-benar dipakai? Kalau ya, rail terang butuh set token
   tersendiri — dokumen ini baru mengatur rail gelap.
