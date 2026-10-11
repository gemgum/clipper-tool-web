# Rencana perombakan tampilan

> **Aturan yang mengikat ada di `CLAUDE.md`, bukan di sini.** Dua hal — jendela
> haram bergulir, dan halaman `/` (Video clips) adalah standar yang disalin
> halaman lain — dipindah ke sana 6 Agustus 2026 setelah dilanggar berkali-kali.
> Sebabnya bukan lupa: berkas ini 800 baris dan tidak dibaca ulang tiap sesi,
> sedangkan `CLAUDE.md` dimuat setiap kali. Aturan yang harus dipatuhi tiap
> giliran TIDAK boleh tinggal di catatan sepanjang ini. Yang di sini adalah
> ALASAN dan hasil pengukurannya.

Ditulis 5 Agustus 2026, **belum dikerjakan**. MVP sudah jalan di Windows, jadi
pekerjaan berikutnya adalah tampilan. Berkas ini menyimpan keputusan yang sudah
diambil dan batasan yang harus diketahui sebelum ada yang menggambar.

## Keadaan sekarang

```
gui/           Next.js, ekspor statis, disajikan engine
               797 baris CSS tulisan tangan · 180 kelas · 294 className
               0 dependensi UI (hanya next + react)
               font: system-ui (berbeda-beda di tiap komputer)
               ikon: emoji
               tidak ada gui/public — nol aset lokal
```

## Keputusan: pindah ke Tailwind

Alasannya bukan Tailwind lebih baik dari CSS tulisan tangan, melainkan
**ekosistem referensinya**. Contoh tampilan yang bisa ditiru hari ini hampir
selalu berbentuk kelas Tailwind; kalau proyeknya CSS sendiri, tiap referensi
harus diterjemahkan dulu — sama mahalnya dengan membuat dari nol. Repo situs
(`gemgum.github.io`) juga sudah memakainya, jadi satu cara berpikir untuk dua
proyek.

**Yang mahal bukan memasangnya, melainkan berhenti di tengah.** Dua sistem yang
hidup berdampingan memaksa tiap suntingan berikutnya memilih "pakai yang mana",
sementara CSS lama terus tumbuh dan aturan mati menumpuk.

Karena itu: **kerjakan bersamaan dengan perombakan tampilan, sekali jalan.**
Perombakan menulis ulang markup-nya toh, jadi migrasinya nyaris gratis.
Memigrasikan lebih dulu berarti menerjemahkan gaya yang sebentar lagi dibuang.

Satu langkah yang membuat masa peralihan bertahan: **pindahkan 10 variabel warna
di `globals.css` jadi token `@theme` lebih dulu**, dengan nilai yang sekarang.
Halaman lama dan baru tetap bicara warna yang sama, jadi tidak terlihat seperti
dua aplikasi. Nilainya tinggal ditimpa angka dari Figma nanti.

## Arah tata letak (ditetapkan 5 Agustus 2026)

Syarat dari pemilik proyek: **setiap fitur harus muat dalam satu jendela, tanpa
menggeser ke atas-bawah maupun kiri-kanan.**

Satu penegasan yang menentukan seluruh sisanya: **JENDELANYA yang tidak boleh
bergulir, bukan setiap kotak di dalamnya.** Daftar 20 klip, kotak log, dan
penjelajah berkas harus bergulir di dalam kotaknya sendiri — itu bukan
pelanggaran, itu justru cara aplikasi desktop bekerja. Yang haram adalah harus
menggulir HALAMAN untuk menemukan tombol.

### Yang dipilih: kerangka berzona tetap

Bingkai luar diam, isinya yang bergerak. Kepala (judul + keadaan) dan panel
samping tetap di tempatnya; daftar panjang bergulir di dalam kotaknya. Pola yang
sama dipakai HandBrake, Lightroom, DaVinci, LM Studio.

Kelasnya: `.screen` → `.screen-head` + `.screen-body` (`.screen-main` +
`.screen-side`), semuanya di `globals.css`.

### Yang DITOLAK, dan alasannya

- **F-shape** bukan sistem tata letak melainkan temuan pola BACA untuk halaman
  berisi teks panjang. Clipper papan kendali, bukan bacaan — ia menjawab
  pertanyaan yang tidak kita punya.
- **Masonry** justru bertabrakan dengan syarat di atas: kolom bertinggi
  bervariasi tumbuh ke bawah tanpa batas, jadi ia MENJAMIN gulir halaman.
- **Neo-Brutalism** memakai warna pekat dan garis tebal sebagai hiasan di
  mana-mana. Clipper penuh KEADAAN (hijau/merah, sedang mengunduh, skor, bilah
  kemajuan); begitu semuanya berteriak, merah galat berhenti berarti galat —
  kebalikan dari keputusan "dasar netral membuat warna jadi sinyal" di bawah.
  Dua sifatnya yang tetap diambil karena memang sejalan: warna solid tanpa
  gradien, dan garis 1 px yang tegas.

### Dua hal yang hanya ketahuan dengan mengukur

1. **`min-height` pada `body` TIDAK cukup.** Dengan `min-height: 100dvh`, body
   tumbuh mengikuti isinya (terukur: 1384 px pada jendela 505 px), jadi `flex: 1`
   tidak punya sisa ruang untuk dibagi — tidak ada yang menyusut, dan gulirnya
   bocor ke halaman. Yang mengunci: `body:has(.screen) { height: 100dvh;
   overflow: hidden }`. Dipagari `:has()` supaya HANYA halaman berkerangka yang
   terkunci; halaman lama tetap bergulir seperti biasa. Sesudahnya:
   `body = 505/505`, `main = 283/1162` — halaman diam, daftarnya bergulir.
2. **`min-height: 0` wajib ada di setiap flex item yang membungkus area
   bergulir.** Tanpa itu, flex item menolak menyusut lebih kecil daripada isinya
   dan seluruh usaha di atas batal.

Tinggi bilah navigasi TIDAK dipakukan sebagai angka. `calc(100dvh - 52px)`
selalu meleset begitu navigasinya disentuh, dan melesetnya muncul sebagai gulir
satu piksel yang sulit dilacak. Karena itu `body` jadi kolom flex dan halaman
memakai `flex: 1` — tingginya terhitung sendiri.

### Diverifikasi

`/requirements` dipindah lebih dulu (halaman paling banyak keadaan, sesuai
saran di bawah). Dipotret di **900×600** (batas terkecil) dan **1240×860**
(ukuran bawaan jendela): jendela tidak bergulir di keduanya, tombol "Check
again" selalu terlihat, dan halaman klip **0 piksel berubah** — kerangkanya
tidak menyentuh halaman yang belum dipindah.

**Ketiga halaman sudah dipindah.** Terukur di jendela 900×600 (tinggi viewport
505 px):

| Halaman | body | isi kolom utama |
| --- | --- | --- |
| `/requirements` | 505/505 | 283 / 1162 |
| `/` klip | 505/505 | 372 / 1493 |
| `/news` | 505/505 | 337 / 337 |

`body = 505/505` di ketiganya: **jendelanya tidak pernah bergulir.** Kolomnya
yang bergulir, dan hanya bila isinya memang lebih panjang.

### Navigasi: rail kiri + setelan di kanan atas

Diminta 5 Agustus 2026 dengan acuan sebuah dasbor: navigasi berpindah dari
bilah tab di ATAS ke **rail vertikal di KIRI**, dan Requirements keluar dari
deretan tab menjadi **setelan di kanan atas**.

Pembedaannya bisa diuji, bukan selera: yang di rail dibuka berkali-kali sehari
(potong video, kartu berita); yang di kanan atas dibuka sekali lalu dilupakan
(komponen, folder, bahasa). Selama ketiganya berjajar sebagai tab, ketiganya
terlihat sama penting.

Kerangkanya `grid-template-areas` di `.app` — rail membentang dua baris penuh,
bilah atas dan halaman berbagi kolom kanan. Grid dipilih karena `<Nav>`
merender rail DAN bilah atas sebagai dua saudara; dengan grid keduanya bisa
ditempatkan tanpa div pembungkus tambahan.

Ikut berubah: judul halaman klip dulu berbunyi "Clipper" (merek). Nama aplikasi
sekarang ada di rail, jadi judulnya menjadi "Video clips" — mengulang merek di
dalam halaman hanya memakan baris tanpa memberi tahu apa pun.

### Keputusan per halaman

- **`/requirements`** — dua kolom. Kiri daftar komponen (yang ditindaklanjuti),
  kanan letak folder + tombol "Check again" (keterangan, jarang disentuh).
- **`/` klip** — dua kolom, DAN tombol **Mulai** dinaikkan ke kepala. Dulu ia
  ada di bawah seluruh setelan, jadi ia hilang dari layar persis ketika pengguna
  selesai menyetel dan ingin menekannya. Bilah kemajuan ikut ke kepala dengan
  alasan yang sama. Kiri: sumber, pratinjau subtitle, log, hasil klip. Kanan:
  setelan render + mesin AI.
- **`/news`** — ~~**satu kolom**, sengaja~~ **DICABUT 6 Agustus 2026, lihat
  bagian "Kartu berita disamakan dengan halaman klip" di bawah.** Alasan lamanya
  (alurnya berurutan, dua kolom memutus urutan) ternyata salah timbang: satu
  kolom BERARTI menggulir halaman, dan itu justru syarat yang paling keras.

Yang BELUM dikerjakan dan masih terasa sesak di 900×600: kotak seret-lepas di
halaman klip memakan tinggi besar padahal ada juga tombol "pilih berkas" dan
kolom path. Itu perapian isi, bukan kerangka.

## Batasan yang harus diketahui desainernya

Empat hal ini sering dilanggar referensi, dan ketiganya baru ketahuan setelah
kode ditulis:

1. **Jendela bisa diubah ukurannya** — minimal 900×600 (`tauri.conf.json`).
   Frame lebar tetap perlu aturan: apa yang melar, apa yang tetap, apa yang
   turun.
2. **Dua bahasa.** Semua teks dari kamus i18n, dan bahasa Indonesia rata-rata
   15–20% lebih panjang. Tombol yang pas untuk "Install" harus tetap muat untuk
   "Pasang komponen".
3. **Tidak ada internet.** Tidak boleh ada satu pun alamat luar di antarmuka —
   bukan font, ikon, gambar, maupun skrip. Ujinya sederhana dan menangkap
   seluruh kelasnya: **cabut jaringan, buka aplikasi.** Kalau ada yang berubah,
   masih ada yang menggantung. (Pengecualian sah: gambar artikel di tab kartu —
   itu isi, bukan tampilan.)
4. **Dua area terkunci ke engine.** Penyeretan jangkar subtitle di ruang
   1080×1920 dan pratinjau kartu harus mencerminkan perhitungan engine PERSIS;
   kalau tidak, yang terlihat berbeda dengan hasil render. Bingkai dan kontrol
   di sekitarnya bebas diubah, angkanya tidak.

## Bentuk serahan Figma yang terpakai

Tautan Figma tidak bisa dibaca agen (butuh login, kanvasnya bukan halaman). Yang
bekerja:

- **PNG 2x per layar**, termasuk keadaan: kosong, sedang berjalan, galat, teks
  panjang. Aplikasi ini sebagian besar waktunya "sedang berjalan" — desain dari
  layar diam biasanya melupakan itu.
- **Token sebagai teks** (warna hex, radius, skala jarak, ukuran & tebal huruf),
  bukan diukur dari gambar.
- **Pemetaan frame → halaman**: `/` klip, `/news` kartu, `/requirements`.

Cakupan yang paling berhasil: **token + dua-tiga layar acuan**, lalu sistemnya
diterapkan ke sisanya. Menyalin pixel-perfect tiap halaman menghasilkan tampilan
yang kaku begitu jendelanya diubah ukuran.

Mulai dari **`/requirements`**: halaman itu paling banyak berisi keadaan
(hijau/merah/sedang mengunduh/gagal), jadi sistem desainnya teruji sejak awal.
Mulai dari halaman paling sepi berarti masalahnya baru ketahuan di halaman
terakhir.

## Aset: harus lokal

Belum ada `gui/public` sama sekali. Yang akan perlu diunduh ke sana:

| Aset | Catatan |
| --- | --- |
| Font antarmuka | `.woff2` + `@font-face`. Jangan tertukar dengan `assets/fonts` — itu font subtitle untuk libass di dalam video. |
| Ikon | lihat di bawah |
| Gambar/ilustrasi | SVG dari Figma |

Isi `gui/public/` ikut ke `gui/out` saat build, lalu disajikan engine — jadi
otomatis masuk ke pemasang.

## Ikon: emoji harus diganti

Bukan soal selera. Emoji ✂️ dan 📁 tampil sebagai **kotak kosong** di jendela
Tauri Linux karena tidak ada font emoji — sudah terbukti, bukan dugaan. Emoji
bergantung pada font sistem, persis hal yang tidak boleh diandalkan aplikasi
yang harus tampil sama di mana-mana.

**Kit yang sudah dikumpulkan** (`~/documents/design_ui`): Xmind Desktop UI Kit
(Community), 172 SVG + 15 PNG. Hasil pembacaan 5 Agustus 2026:

- Hanya **±19 ikon** yang relevan untuk Clipper (play, pause, folder, camera,
  close, check, warning, update, reset, add, clock). 119 berkas di antaranya
  soal peta pikiran — cabang, boundary, struktur — yang tidak ada padanannya di
  sini.
- Warna dipaku `stroke="#27292A"` (abu gelap): di antarmuka gelap Clipper nyaris
  tak terlihat. Perlu diganti `currentColor`.
- Ukuran 16×16, stroke 1.5, konsisten — asal ikon dari sumber lain mengikuti.
- **Lisensi — SUDAH DIPERIKSA, dan ternyata tidak perlu diandalkan.** Berkas
  gratis di Figma Community berlisensi CC BY 4.0 secara bawaan, yang memang
  mengizinkan pemakaian komersial dan perubahan asalkan pembuatnya dikredit.
  Tapi yang kita ambil dari kit ini hanyalah **gaya, palet, dan aturan tata
  letak** — dan itu ide serta angka, bukan ekspresi berhak cipta. Diperiksa 5
  Agustus 2026: **tidak ada satu pun berkas dari kit ini di repo.** Ikonnya
  datang dari lucide-react, fontnya OFL dari Google Fonts.

  Karena tidak ada aset mereka yang dikirim, **tidak ada yang perlu diatribusi**
  — jadi tidak ada berkas kredit, dan tidak ada kewajiban yang harus dijaga
  saat rilis.

  Yang tetap dijauhi, sebab CC BY memang tidak mencakupnya: **merek** (nama dan
  logo Xmind) dan **font NeverMind**, yang di kit itu diberi tombol unduh
  terpisah — pola yang biasanya berarti lisensinya sendiri.

  Kalau suatu saat aset kit-nya benar-benar dipakai, syaratnya berubah: wajib
  ada kredit ke pembuatnya, tautan ke lisensinya, dan pernyataan bahwa ada yang
  diubah.

Pembanding: **lucide-react**, lisensi ISC (bebas komersial), satu dependensi,
gaya seragam, menutup seluruh kebutuhan Clipper (gunting, unduh, folder, berkas,
putar, setelan, centang, peringatan, galat, muat-ulang, tautan-luar, panah,
hapus, tambah), dan sudah dipakai repo situs.

Saran: pakai set lengkap sebagai dasar, ambil dari kit lain hanya bila ada
bentuk yang benar-benar pas dan tidak ada padanannya.

## Gaya & warna (ditetapkan 5 Agustus 2026)

Acuannya kit Xmind. Warna diambil dengan mengukur gambarnya, bukan dikira-kira.

### Gayanya: flat 2.0

Bukan flat murni, bukan Material. Buktinya dari gambar acuan:

- **ada bayangan, tapi sangat halus** — tepi kartu melandai
  `#FEFEFE → #FDFDFD → #FBFBFB → #F7F7F7 → #F3F3F3 → #ECECEC` selebar ±12 px
  sebelum garisnya; panel biasa tidak berbayang sama sekali;
- **warna solid tanpa gradien** — tombol biru tetap `#0072EC` dari atas ke bawah.

Ciri lain yang ikut: dasar hampir tanpa warna (putih 71%, abu 11%), ikon garis
16×16 stroke 1,5, pemisah berupa garis rambut 1 px, dan warna dipakai untuk
MAKNA — biru = interaktif, deret warna = kategori.

Satu kalimat untuk desainer: *flat 2.0, dasar netral, satu aksen biru, ikon
garis, kedalaman hanya untuk elemen yang melayang.*

Cocok untuk Clipper bukan karena selera: aplikasi ini penuh **keadaan**
(hijau/merah/sedang mengunduh, skor, bilah kemajuan). Dasar yang netral membuat
warna jadi sinyal — kalau ada yang hijau, itu berarti sesuatu.

### Palet terang

| Token | Gelap (sekarang) | Terang |
| --- | --- | --- |
| `--bg` | `#0f1115` | `#F5F5F5` |
| `--panel` | `#171a21` | `#FFFFFF` |
| `--panel2` | `#1e222b` | `#FAFAFA` |
| `--border` | `#2a2f3a` | `#E2E2E2` |
| `--text` | `#e6e9ef` | `#27292A` |
| `--muted` | `#9aa4b2` | `#6B6E70` |
| `--accent` | `#4f8cff` | `#0072EC` |
| `--good` | `#3ecf8e` | `#20C07D` |
| `--warn` | `#f5a623` | `#FF9D43` |
| galat | `#ff6b6b` | `#FF4747` |

Warna kategori (dari deret Tag & Priority): `#FF4747` `#FF9D43` `#F7CF00`
`#20C07D` `#577CFF` `#6A4CCC`.

**Merah `#F9423A` adalah LOGO Xmind, bukan warna aksinya.** Yang dipakai untuk
tombol aktif dan isian terpilih di kit itu adalah biru `#0072EC`. Memakai merah
sebagai aksen utama akan bentrok dengan warna galat — pengguna tidak bisa
membedakan "tombol utama" dari "ada yang salah".

### Kontras — yang tidak boleh jadi teks

Diukur di atas putih (ambang 4,5:1 untuk teks biasa):

| | Rasio | Boleh untuk |
| --- | --- | --- |
| `#27292A` | 14,6:1 | semua |
| `#6B6E70` | 5,1:1 | semua |
| `#0072EC` | 4,6:1 | semua (pas-pasan) |
| `#FF4747` | 3,4:1 | teks besar / ikon |
| `#A8A9A9` | 2,4:1 | **jangan untuk teks** |
| `#20C07D` | 2,4:1 | **jangan untuk teks** |
| `#FF9D43` | 2,1:1 | **jangan untuk teks** |

Di tema gelap hijau & oranye aman sebagai teks; di tema terang **tidak**. Di
sana keduanya hanya boleh jadi lampu status — titik, bilah, latar. Ini jenis
kesalahan yang lolos dari mata saat mendesain dan baru terasa setelah dipakai
lama.

### Keputusan

- **Dua tema, terang dan gelap.** Ditetapkan sejak awal supaya tidak perlu
  menyisir ulang semua halaman nanti; struktur tokennya sudah siap.
- **Area pratinjau belum diputuskan.** Kalau nanti harus dibedakan dari
  sekitarnya, coba dulu bentuk melayang (kartu berbayang) sebelum memaksanya
  tetap gelap.

## Bug yang ditemukan, dikerjakan bersama pembaruan tampilan

Dilaporkan 5 Agustus 2026 dari hasil render sungguhan di Windows.
**Nomor 1, 2, dan 3 sudah SELESAI hari yang sama.**

Cara mengukurnya, supaya bisa diulang: `.ass` dibuat lewat penulis engine
sendiri, dirender `ffmpeg -f lavfi -i color=black:s=1080x1920 -vf
subtitles=…:fontsdir=…`, dan pratinjaunya direproduksi 1:1 di Chrome headless
pada kanvas 1080×1920 yang sama. Keduanya lalu dibandingkan per piksel. Tanpa
mengukur di ruang yang SAMA, semua selisih di bawah ini tidak akan terlihat.

### 1. Subtitle hasil render tidak memakai Montserrat SAMA SEKALI — SELESAI

Dugaan awal di catatan ini ("libass merender instance bawaan Thin") **meleset,
dan yang sebenarnya lebih buruk.** Diukur 5 Agustus 2026:

`assets/fonts/Montserrat.ttf` memang font variabel (wght 100…900, bawaan 100),
tapi masalahnya bukan bobot melainkan **nama**: berkas itu menyebut dirinya
family **"Montserrat Thin"**, sedangkan engine menulis `Fontname: Montserrat` ke
`.ass`. libass karena itu tidak pernah mengenalinya.

Buktinya tidak bisa dibantah: render dengan berkas itu di `fontsdir` **identik
byte demi byte** dengan render tanpa font sama sekali.

```
fontsdir berisi Montserrat[wght].ttf → sha256 1ff7985c…
fontsdir kosong                      → sha256 1ff7985c…
tanpa fontsdir                       → sha256 1ff7985c…
```

Jadi setiap subtitle yang pernah dirender proyek ini memakai **font cadangan
sistem**, bukan Montserrat — dan di mesin pengguna yang font sistemnya berbeda,
hasilnya berbeda pula.

**Perbaikan yang dikerjakan:** `fetch-fonts.sh` kini mengunduh **dua face
statis** (`Montserrat-Regular.ttf` + `Montserrat-Bold.ttf`, dipaku ke tag
`v7.222` repo hulu) di samping yang variabel. Yang variabel TETAP ada dan tetap
dipakai kartu berita — kartu dirender Chrome, yang memang memahami sumbu
variabel. Ketiganya hidup berdampingan di satu folder tanpa saling ganggu
(diuji).

**Kenapa dua face, bukan satu** — diukur pada teks yang sama:

| Isi `assets/fonts` | Bold=1 | Bold=0 |
| --- | --- | --- |
| Regular saja | 3.755 tinta (penebalan buatan, terlalu tipis) | — |
| Bold saja | 6.056 | 6.056 (togglenya mati) |
| **Regular + Bold** | **6.056** | **3.097** |

**Jangan memakai `Montserrat-SemiBold`**: family di dalamnya "Montserrat
SemiBold", jadi ia tidak akan pernah cocok dengan "Montserrat" — persoalan yang
sama dengan yang variabel.

Anton dan Bebas Neue sudah statis, dan keduanya memang hanya terbit satu bobot.

### 2. Pratinjau menggambar subtitle 38% TERLALU BESAR — SELESAI

Urutan "perbaiki font dulu, baru ukur" ternyata benar dan menyelamatkan: setelah
nomor 1 beres, selisihnya bukan "sedikit lebih naik" melainkan **selisih
UKURAN**, dan besarnya berbeda tiap font.

Sebabnya satu kalimat: **`.ass` mengartikan ukuran font sebagai tinggi KOTAK
font (`winAscent + winDescent`), CSS mengartikannya sebagai ukuran em.**

Diukur, bukan diturunkan dari teori — `Fontsize` di `.ass` disapu sampai lebar
tintanya sama dengan `font-size: 72px` di Chrome:

```
Fontsize 72 →  lebar 352      Fontsize 96 →  lebar 470
Fontsize 92 →  lebar 450      Fontsize 99 →  lebar 484   ← 72px Chrome = 485
```

72/99 = 0,727, dan `upem/(winAscent+winDescent)` Montserrat = 1000/1379 = 0,725.
Cocok. Angka itu berbeda tiap font: **Montserrat 0,725 · Bebas Neue 0,769 ·
Anton 0,577 · DejaVu Sans 0,859** — jadi ia tidak bisa jadi satu tetapan di GUI.

Akibat kedua dari definisi yang sama: **jarak antarbaris libass persis sebesar
`Fontsize`**, berapa pun fontnya (diukur: dua baris ukuran 72 berjarak 72 px).
Faktor `1.17` yang dulu ada di GUI diukur ketika font bawaan tidak pernah
benar-benar dipakai — jadi ia mengukur font cadangan sistem.

**Perbaikan yang dikerjakan:** engine membaca `head.unitsPerEm` dan
`OS/2.usWinAscent/Descent` dari berkas fontnya sendiri
(`engine/internal/api/fontmetrics.go`, tanpa pustaka luar) dan melaporkannya
sebagai `scale` di `/api/fonts` dan `/api/font-check` — untuk font bawaan MAUPUN
font sistem. GUI memakainya: `font-size = size × scale`,
`line-height = 1/scale`, `blockH = size × barisan` (bukan `× 1.17`).

Hasilnya, diukur dengan render libass dan render Chrome berdampingan di ruang
1080×1920 yang sama:

| | sebelum | sesudah |
| --- | --- | --- |
| selisih lebar tinta | +133 px | **0 px** |
| selisih tepi kiri/kanan | −65 / +68 px | **0 / 0 px** |
| selisih tepi atas | — | −4 px |

**Sisa −4 px belum ditutup**, dan sengaja tidak ditambal: itu selisih cara
browser dan libass menaruh garis dasar di dalam kotak baris. Pada pratinjau
setinggi 480 px, 4 px di ruang 1920 = **1 piksel layar**. Kalau nanti mau
dihabiskan, jalannya menjangkarkan teks pratinjau pada garis dasar (butuh
`winAscent` dikirim juga), bukan menambah offset karangan.

### 2b. Pratinjau memuat face tebal yang sama dengan render

Bagian dari nomor 2 dan ikut selesai: GUI dulu memasang SATU `@font-face` tanpa
penyebut bobot, lalu memakai `font-weight: 800`. Browser karena itu menebalkan
sendiri face tegaknya — penebalan buatan yang tidak sama dengan face Bold
sungguhan yang dipakai libass.

Sekarang `/api/font-file` menerima `?weight=`, GUI memasang dua `@font-face`
(400 dan 700), dan `.suboverlay` memakai `font-weight: 700` — sama dengan arti
`Bold=1` di `.ass`.

### 3. Grid untuk menggeser subtitle — SELESAI

Permintaan: menggeser subtitle terasa tidak pasti. Tambahkan grid supaya
penempatannya bisa diulang, **tetap dengan sumbu X dan Y yang terlihat**.

Ukurannya tidak diputuskan di sini melainkan **dijadikan pilihan**, sebab "rasa"
grid cuma bisa dinilai dengan mencoba: `mati · 10 · 20 · 24 · 40`, bawaan **20**
(54×96 kotak). Pilihannya ada di bilah tombol pratinjau, sebelah "garis tengah
terus".

Yang berlaku sekarang:

- **magnet ke garis tengah MENANG atas grid.** Ini bukan selera: titik tengahnya
  sering bukan kelipatan grid — pada grid 24, X tengah 540 tidak terjangkau sama
  sekali (540/24 = 22,5), dan pada grid 20 jangkar tengah Y 888 juga tidak. Kalau
  grid dipasang lebih dulu, "tepat di tengah" jadi mustahil dicapai — persis
  kemampuan yang catatan ini minta jangan dibuang;
- **Alt = abaikan grid**, dan **tombol panah menggeser 1 px** (Shift = 10).
  Overlay subtitle karena itu `tabIndex={0}` — panahnya butuh fokus;
- **sumbu X/Y digambar pada posisi subtitle sekarang**, warna aksen supaya
  berbeda dari garis tengah yang putih, **berikut angkanya** (`540 · 888`) selama
  diseret. Tanpa angka, "pasti" hanya terasa dan tidak bisa diulang di klip
  berikutnya.

Dua keputusan kecil yang diambil saat mengerjakan:

- **Mesh grid tidak digambar di bawah 20.** Pratinjau selebar 270 px berarti
  kotak grid 10 hanya 2,5 px di layar — yang terlihat bukan grid melainkan kabut
  kelabu di atas frame videonya. Menempelnya tetap jalan; cuma gambarnya yang
  tidak menolong.
- **Mesh-nya satu elemen berlatar berulang**, bukan puluhan `<div>`: pada grid 10
  itu 108×192 kotak, dan menggambarnya satu per satu berarti ratusan elemen yang
  dihitung ulang tiap gerakan seretan.

## Yang bisa dikerjakan tanpa menunggu desain

Tiga hal ini tidak menuntut keputusan desain, dan nilainya tidak hangus walau
rancangannya berubah:

1. ~~pasang Tailwind + pindahkan 10 warna jadi token `@theme`~~ — **SELESAI**;
2. ~~ganti emoji dengan ikon sungguhan~~ — **SELESAI**;
3. buat `gui/public/` beserta aturan "nol alamat luar" — **ditunda, lihat bawah**.

### 1. Tailwind + token warna — selesai

Tailwind v4 lewat `@tailwindcss/postcss`; tidak ada `tailwind.config.js`,
seluruh setelannya di CSS. Sepuluh warna jadi token `@theme`
(`--color-bg`, `--color-panel`, …) dan nama pendek lama (`--bg`, `--panel`, …)
dipetakan ke token itu — satu tempat, satu kebenaran, dan CSS tulisan tangan di
bawahnya tidak disentuh sama sekali. Itu memang maksudnya: perombakan akan
menulis ulang markupnya, jadi menerjemahkan gaya lama sekarang adalah pekerjaan
yang hangus dua kali.

**Tailwind diimpor TANPA preflight**, dan ini ditemukan lewat pengukuran, bukan
kehati-hatian: `@import "tailwindcss"` yang biasa menggeser **12,38% piksel**
halaman (tombol kehilangan tebalnya, seluruh kolom turun ~12 px). Dengan
mengimpor `theme.css` + `utilities.css` saja, selisihnya **0,00% — nol piksel**.
Preflight dinyalakan nanti bersama perombakan, ketika markupnya memang ditulis
ulang dan pergeseran itu tidak jadi kejutan.

### 2. Emoji → ikon — selesai

`lucide-react` (ISC, 1 dependensi) seperti yang direncanakan. Yang berubah:
`gui/app` kini memuat **nol** lambang yang butuh font emoji.

Yang penting diketahui sebelum menambah lambang baru: **tidak semua emoji
bermasalah.** Yang butuh font emoji adalah lambang dengan `Emoji_Presentation=Yes`
— ✅ ⛔ ⬇ ⬆ 🔄 👁 🔎 📋 📁 🎬 🔗 ✏️ ✂️. Sedangkan **⚠ ✓ ✕ → ↗ ↓ ↑ ✗ adalah simbol
teks biasa** yang ada di font mana pun, dan semuanya DIBIARKAN. Mengganti yang
tidak perlu hanya menambah komponen tanpa memperbaiki apa pun.

Karena itu pekerjaannya terbelah dua, dan pembelahan itu bukan pilihan
melainkan keharusan:

- **Label** (tombol, status) → komponen ikon di JSX, lambangnya dibuang dari
  kamus i18n;
- **Baris log** → tetap teks, sebab log adalah `string[]` yang dirender sebagai
  `<div>{teks}</div>` dan tidak bisa memuat komponen. Lambangnya diganti yang
  aman: ✅→✓, ⬇→↓, ⬆→↑, 🔄→↻, dan 🔎/📋/🎬 dibuang begitu saja;
- **Isi `<option>`** tidak bisa memuat komponen sama sekali (`fontOther`,
  daftar model) — di situ lambangnya dibuang atau memakai ✓/✗ yang aman.

Biayanya 2 kB pada bundel halaman utama.

Cara memeriksa aturannya masih dipegang — hasil kosong berarti aman:

```bash
grep -rnP '[\x{1F300}-\x{1FAFF}\x{2705}\x{26D4}\x{2B06}\x{2B07}\x{270F}\x{2702}\x{23F0}-\x{23FA}]' \
  gui/app --include=*.tsx --include=*.css
```

Rentang `23F0–23FA` ditambahkan 6 Agustus 2026: `⏹` (U+23F9) lolos dari daftar
lama padahal `Emoji_Presentation=Yes`, dan ia sudah terlanjur duduk di tombol
Batalkan serta di baris log selama berbulan-bulan. Kalau ada lambang baru yang
mencurigakan, periksa propertinya dulu — jangan menebak dari bentuknya.

### 3. `gui/public/` — ditunda, dan alasannya

Belum dibuat, sebab **tidak ada yang perlu ditaruh di sana**: ikon sekarang
datang dari lucide (komponen JS, bukan berkas), dan tidak ada gambar. Folder
kosong tidak bisa dilacak git, jadi membuatnya sekarang berarti menambah nol.

Aturan "nol alamat luar" sendiri **sudah dijaga mesin** sejak `notes/30`: header
`Content-Security-Policy` yang disajikan engine menolak skrip, gaya, dan font
dari host luar, jadi satu tautan CDN yang tak sengaja ikut akan gagal memuat
alih-alih diam-diam bekerja di mesin yang punya internet lalu putus di mesin
yang tidak.

Yang membuat folder ini benar-benar diperlukan adalah **font antarmuka**, dan
itu belum bisa diputuskan agen: antarmuka masih memakai `system-ui`, yang
persis penyakit yang sama dengan emoji — tampilannya berbeda di tiap komputer.
Begitu fontnya dipilih (dari Figma atau sekarang), `gui/public/` dibuat bersama
berkas `.woff2`-nya, tidak sebelum itu.

## Kenapa tiga percobaan tata letak gagal, dan apa yang harus dikerjakan dulu

Ditulis 5 Agustus 2026 setelah GAGAL TIGA KALI menyusun kolom di halaman klip.

Gejalanya selalu sama: blok dipindah dengan memotong teks di `page.tsx`, `tsc`
lolos, build lolos — lalu di browser paneL mendarat **tersarang di dalam panel
lain**, bukan di sebelahnya. Diukur: `.split` berisi 1 anak padahal 2 ditulis;
`.screen-body` berisi 2 anak padahal 3 ditulis.

**Sebabnya bukan detail, melainkan cara kerjanya.** `page.tsx` 1.300 baris
dengan belasan tingkat bersarang. Memindahkan blok di dalamnya = memotong ribuan
karakter berdasarkan penanda komentar, dan **tidak ada yang bisa memverifikasi
hasilnya**: TypeScript hanya memeriksa tag seimbang, bukan tag berada di induk
yang benar. Menghitung kedalaman dengan regex juga gagal — tag menutup-sendiri
ikut terhitung.

**Yang harus dikerjakan LEBIH DULU, sebelum menyentuh kolom lagi:**

pecah `page.tsx` jadi komponen bernama, **satu per satu, build di antaranya**:

| Komponen | Isi |
| --- | --- |
| `<PreviewPanel>` | bingkai 9:16, seretan subtitle, kendali huruf & posisi |
| `<FramePanel>` | panduan platform + cara video masuk bingkai + zoom |
| `<RenderSettings>` | mode, model, resolusi, kualitas, fps, durasi, jumlah, simpan |
| `<SourcePanel>` | seret-lepas, path video, folder keluaran |
| `<AiEnginePanel>` | mesin skor, model, koreksi transkrip, daftar istilah |
| `<RunPanel>` | tombol Mulai/Batal + bilah kemajuan |
| `<LogPanel>` | kotak log |

Sesudah itu menyusun tiga kolom cuma soal menaruh tiga tag berdampingan —
salah tempat langsung kelihatan di berkas sepanjang 20 baris, bukan 1.300.

### Dikerjakan 6 Agustus 2026 — dan alat yang akhirnya bisa memverifikasi

Catatan di atas menyebut "tidak ada yang bisa memverifikasi hasilnya". Ternyata
ada, dan sudah terpasang di repo sejak awal: **parser TypeScript itu sendiri**
(`gui/node_modules/typescript`). `ts.createSourceFile(..., ScriptKind.TSX)` lalu
menelusuri `JsxElement`/`JsxSelfClosingElement` mencetak pohon induk-anak yang
sebenarnya — tag menutup-sendiri terurus otomatis, dan `{kondisi && ...}` serta
fragmen dilewati tanpa menambah kedalaman, persis seperti DOM. Itulah yang
seharusnya dipakai dari percobaan pertama, bukan menghitung indentasi.

Yang ditemukannya begitu dijalankan: **`.screen-body` berisi 5 anak, bukan 3.**
Panel Berkas, panel Mesin AI, dan tombol Mulai memang salah induk seperti
diduga — tapi ada satu lagi yang belum pernah disadari, **kotak log tersarang DI
DALAM panel pratinjau**, sebab `</div>` penutup panelnya hilang satu.

Ada juga yang hilang diam-diam di salah satu dari tiga percobaan itu: **kotak
seret-lepas berikut tombol "pilih berkas"**. Markup-nya lenyap, tapi `dragOver`,
`uploading`, `uploadPct`, dan `useFile` semuanya tertinggal sebagai kode mati —
tak satu pun compiler mengeluh, karena secara tipe semuanya masih sah.
Dikembalikan dari `git show` ke dalam `<SourcePanel>`.

Sesudah dipecah, susunan kolomnya muat dalam satu layar `page.tsx` (1.286 → 547
baris), dan diperiksa **di HTML hasil ekspor**, bukan di sumbernya:

```
.screen-body  children=2   → .screen-main · .screen-col
.screen-main  children=3   → SourcePanel · panel-grow(Preview+Frame) · LogPanel
.screen-col   children=3   → RenderSettings · AiEnginePanel · RunPanel
```

Dua keputusan kecil saat memecah:

- **`<FramePanel>` masuk lewat `children` `<PreviewPanel>`.** Ia memang duduk di
  ujung kolom setelan pratinjau, dan menaruhnya di sana lewat slot menjaga DOM
  tetap sama persis — pemecahannya tidak menggeser satu piksel pun.
- **Panel yang meregang ditunjuk, bukan diurutkan.** `.screen-main > .panel:
  first-of-type { flex: 1 }` dulu benar karena pratinjau kebetulan berdiri
  paling atas; sejak Berkas naik ke atasnya, kolomnya menandai sendiri lewat
  `.panel-grow`. Urutan panel jadi boleh berubah tanpa diam-diam memindahkan
  ruang lebih ke panel yang salah.

Yang **belum** dikerjakan dari bagian ini: gaya tiap komponen masih memakai
kelas `globals.css` yang lama, belum pindah ke Tailwind. Memecah dan menerjemahkan
gaya sekaligus berarti dua perubahan besar tanpa titik verifikasi di antaranya —
persis pola yang menghasilkan tiga kegagalan di atas.

## Memadatkan halaman klip (6 Agustus 2026)

Diminta sesudah pemecahan komponen: **jangan sampai pengguna harus menggulir ke
bawah.** Yang dikerjakan, berikut alasan tiap potongannya — sebab tanpa alasan
yang tertulis, tiap sesi berikutnya cenderung mengembalikannya.

### Susunan sekarang: dua panel, titik

```
.screen-main                     .screen-col
└── .panel.panel-grow            ├── .panel        (SetupPanel)
    ├── .source-row              └── .panel.start-panel (RunPanel)
    └── .sub-layout
        ├── .sub-preview         (bingkai 9:16 + kendali)
        └── .sub-settings        (huruf, posisi, + FramePanel)
└── LogPanel (hanya bila ada log)
```

### Yang dibuang, dan kenapa boleh

- **Kotak seret-lepas (~110 px).** Ia mengulang persis apa yang sudah bisa
  dilakukan kolom path di bawahnya, dan tingginya penyumbang tunggal terbesar
  yang memaksa gulir. Menyeret berkas TIDAK hilang: **`.source-row` seluruhnya
  jadi sasaran lepasnya**, dengan garis putus-putus yang cuma muncul saat ada
  yang diseret ke atasnya. Kotak permanen bersambungan dengan tinta tanpa kabar.
- **Panel "Berkas" sebagai panel sendiri.** Sumber pindah jadi baris di kepala
  panel pratinjau — di situlah tempatnya secara sebab-akibat: ia yang menentukan
  gambar tepat di bawahnya. Hemat satu bingkai + satu jarak antarpanel.
- **Panel "Render settings" dan "AI engine" digabung jadi `<SetupPanel>`.**
  Keduanya menjawab pertanyaan yang sama — "job ini dijalankan bagaimana" — dan
  mesin skor kini menempel di kelompok Engine, bersama mode & model whisper. Dua
  bingkai + dua judul panel yang hilang ≈ 90 px.
- **Judul panel** (`clipAppearance`, `renderSettings`, `aiEngine`). Judul
  kelompok di dalamnya sudah menamai isinya, dan bingkai 9:16 mengatakan sendiri
  apa itu. Dua tingkat hierarki untuk satu panel adalah satu tingkat terlalu
  banyak.
- **Tujuh baris keterangan** (`previewHintVideo`, `previewHintNoVideo`,
  `previewNote`, `gridHint`, `zoomFullNote`, `fitWholeNote`,
  `fitWholeCropNote`, `engineNoFallback`). Semuanya menjelaskan apa yang sudah
  terlihat di pratinjau sebelahnya.
- **Kalimat status Ollama yang berbunyi "siap".** Keadaan baik tidak butuh
  kalimat. Yang tersisa hanya keadaan yang MENUNTUT TINDAKAN — tidak jalan,
  belum terpasang, model kurang mampu — dan itu tetap lengkap dengan tombolnya.

**23 kunci i18n** jadi tak terpakai dan ikut dibuang dari kedua kamus (dicek
dengan menyisir seluruh `app/**/*.tsx`, bukan dengan ingatan). Lima di antaranya
sudah mati sejak sebelum ini: `brandTagline`, `download`, `fitFace`, `termsNote`.

### Yang dirapatkan

| | Sebelum | Sesudah |
| --- | --- | --- |
| `.screen .panel` padding | 20 | 16 |
| `.screen .panel` margin-bottom | 12 (+ `gap:12` kolom = 24 px terpakai) | 0 |
| `.group` margin-bottom | 18 | 14 |
| `.group-title` padding+margin | 6 + 12 | 4 + 10 |

Margin bawah `.panel` itu bug diam-diam: kolomnya flex ber-`gap`, jadi margin
ikut DIJUMLAHKAN di atas gap — 24 px terpakai untuk memisahkan dua kotak yang
cukup dipisah 12.

### Tinggi bingkai ikut tinggi jendela

`.sub-preview .preview9x16` dulu dipaku 400 px. Itu angka terbesar yang
sendirian memaksa gulir di jendela terkecil. Sekarang:

```css
--pvh: clamp(240px, 42dvh, 400px);
height: var(--pvh);
```

**Satu variabel untuk keduanya, dan itu wajib**, bukan kerapian: ukuran huruf
pratinjau dihitung sebagai pecahan dari `--pvh` (`calc(size × scale / 1920 ×
var(--pvh))`). Kalau `height` dan `--pvh` jadi dua angka terpisah, keduanya bisa
melenceng dan pratinjau berhenti mencerminkan hasil render — pelanggaran batasan
"dua area terkunci ke engine" di atas.

Kalau tingginya dinaikkan lagi, **lebar kolom di `.sub-layout` harus ikut naik**
(rasio 9:16 — tinggi 400 berarti lebar 225 pada kolom 240 px).

### Sejauh mana gulirnya benar-benar hilang — belum diukur

Angka di bawah **hasil hitungan dari CSS, bukan pengukuran**: tidak ada
Chrome/Chromium di mesin tempat ini dikerjakan, jadi yang bisa diperiksa hanya
struktur DOM di `gui/out/index.html`, bukan tingginya.

| Jendela | Perkiraan tinggi kolom kiri | Ruang tersedia |
| --- | --- | --- |
| 1240×860 (bawaan) | ~590 px | ~690 px — **muat** |
| 900×600 (terkecil) | ~590 px | ~430 px — **kolom kiri masih bergulir** |

Sebabnya struktural, bukan kurang rapat: di 900 px lebar, kolom setelan subtitle
tinggal ~180 px sehingga kendalinya WAJIB menumpuk satu per baris (~480 px), dan
bingkai 9:16 tetap butuh tingginya sendiri. Merapatkan lagi tidak akan menutup
selisih 150 px itu.

Dan itu **tidak melanggar** keputusan di bagian atas catatan ini: yang haram
adalah HALAMAN yang bergulir, bukan kolom. `body` tetap 505/505 — yang bergulir
`.screen-main`, di dalam kotaknya sendiri, persis seperti aplikasi desktop pada
umumnya.

### Penggeser diganti tombol −/+ (6 Agustus 2026, dari tangkapan layar)

Keluhannya satu kalimat — "sangat terlihat berantakan" — dan penyebabnya
kelihatan begitu ditunjuk: **`<input type="range">` melar selebar wadahnya.**
Tiga di antaranya (Ukuran, Garis tepi, Zoom) karena itu membentang penuh dari
tepi ke tepi panel, masing-masing memakai satu baris utuh untuk menyampaikan
SATU ANGKA, sementara `<select>` di atasnya cuma separuh lebar. Yang terbaca
mata bukan "ada tiga setelan", melainkan tiga garis panjang yang tidak sejajar
dengan apa pun.

Dua cacat lain yang ikut hilang, dan keduanya bukan soal rupa:

- **Penggeser tidak pernah menyebut nilainya sendiri.** Angkanya harus
  dititipkan ke labelnya — `"Size ({n})"`, `"Zoom ({n}%)"` — jadi label berhenti
  jadi nama dan berubah jadi tampilan nilai.
- **Nilai yang persis mustahil disetel dengan tetikus.** Menaruh zoom di 85%
  atau ukuran di 72 adalah tebak-tebakan seukuran satu piksel layar; dengan
  tombol −/+ tiap ketukan adalah satu langkah yang bisa dihitung.

`<Stepper>` (`gui/app/stepper.tsx`): satu kotak berbingkai berisi tombol −,
isian angka, dan tombol +. Yang perlu diketahui sebelum menyentuhnya:

- **Tombolnya ikon lucide, bukan huruf.** Tanda minus yang benar (U+2212) tidak
  ada di semua font, sedangkan hubung ASCII terlihat lebih pendek daripada
  plusnya — dua tombol sejajar jadi tampak tidak sama besar. Ini penyakit yang
  sama dengan emoji, cuma lebih halus.
- **Panah naik-turun bawaan `type="number"` dimatikan.** Ia mengulang persis dua
  tombol di kiri-kanannya, dengan sasaran klik seukuran 8 px.
- **Angka yang diketik tangan dibulatkan ke kelipatan `step`**, jadi ia tidak
  pernah mendarat di nilai yang tombolnya sendiri tidak bisa capai.
- **Yang TETAP penggeser: waktu frame pratinjau.** Itu menyusuri durasi video —
  mencari, bukan menyetel — dan menyeret memang cara yang benar untuk itu.

Ukuran dan Garis tepi kini berbagi satu baris; Zoom ikut baris "cara video masuk
bingkai". Keterangan `zoom-ends` ("5% · kecil" / "100% · isi penuh") dibuang:
tombol −/+ mati sendiri begitu sampai batas, jadi ujung skalanya tak perlu
ditulis.

### Kotak log kembali, dan sekarang selalu ada

Hilang tanpa disadari saat panelnya dipecah: `<LogPanel>` mengembalikan `null`
selama `logs` kosong, jadi sebelum job pertama dijalankan halaman ini sama
sekali tidak punya tempat untuk memantau jalannya pekerjaan — dan begitu baris
pertama masuk, kotak yang menyembul menggeser seluruh kolom.

Sekarang ia **selalu dirender**, di bawah panel pratinjau, dengan kalimat
pengganti saat masih kosong. Tingginya **tetap 132 px, bukan `max-height`** —
kotak yang tumbuh mengikuti isinya menggeser kolom tiap baris log masuk, dan log
paling ramai persis ketika pengguna sedang mengawasi hal lain. Isinya bergulir
di dalam kotaknya sendiri; itu memang satu-satunya kotak di kolom ini yang boleh.

### Judul "Video clips" dibuang

Nama halaman sudah tertulis di rail kiri dalam keadaan terpilih. Mengulangnya
sebagai `<h1>` memakan satu baris penuh tanpa memberi tahu apa pun — persoalan
yang sama dengan judul panel di bagian sebelumnya, satu tingkat lebih tinggi.

Ikut berubah: **`.screen-head` tidak dirender sama sekali kalau isinya kosong.**
`.screen` adalah kolom flex ber-`gap`, jadi div kosong pun tetap menyisakan
12 px. Kepala hanya muncul saat ada kemajuan job, galat, atau peringatan model.

### Satu jebakan CSS yang sudah menggigit sekali

`.screen-main > .panel:first-of-type { flex: 1 }` menunjuk panel berdasarkan
URUTAN. Begitu kotak log pindah ke bawah pratinjau, "yang pertama" berubah arti
dan ruang lebih mendarat di panel yang keliru — persis kelas kesalahan yang sama
dengan salah induk, cuma di CSS.

Aturannya sekarang berpasangan dan **sengaja sama kuat (0,4,0)**, dengan yang
menunjuk nama ditaruh belakangan supaya ia menang walau panel yang ditunjuk
kebetulan juga yang pertama:

```css
.screen-main:has(> .panel-grow) > .panel:first-of-type { flex: none; }
.screen-main:has(> .panel-grow) > .panel-grow          { flex: 1; }
```

### Kolom setelan disusun ulang: kisi tiga kolom (6 Agustus 2026)

Keluhan berikutnya — "kurang proper, membingungkan, **bukan tentang penamaan
tapi setiap fungsi**" — menunjuk dua penyakit sekaligus.

**Satu: tepi kanan bergerigi.** Tiap baris memakai `.row` (flex) dan membagi
lebarnya SENDIRI, jadi baris berisi dua kendali melebar sementara baris berisi
tiga menyempit. Tidak ada satu kendali pun yang sejajar dengan kendali di baris
atas atau bawahnya.

Sekarang seluruh kolom memakai satu `.grid3` — `repeat(3, minmax(0,1fr))`.
Kolomnya ditentukan sekali, jadi semua kendali berdiri di garis yang sama
berapa pun isi tiap selnya. Tiga hal yang membuatnya benar-benar rata:

- **`.sub-settings` jadi container sendiri** (`container-type: inline-size`).
  Kisinya harus runtuh menurut lebar KOLOM SETELAN, dan lebar itu sudah dikurangi
  240 px bingkai pratinjau di sebelah kiri — bertanya ke `.screen-main` selalu
  memberi angka yang terlalu besar.
- **`align-items: start` + `align-self: stretch` pada sel tombol/centang.** Kisi
  dibiarkan `start` supaya sel Font yang tinggi (ada isian font manual di
  bawahnya) tidak menarik tetangganya ikut memanjang; sel yang isinya centang
  atau tombol dikembalikan ke `stretch` supaya isinya bisa didorong ke DASAR sel
  dan sejajar dengan kotak isian di kiri-kanannya.
- **`.position-value` dibingkai seperti kotak isian**, dengan padding yang sama
  persis (9+9+1 px), bukan tinggi yang dipaku — tinggi tetap selalu meleset
  begitu ukuran huruf disentuh. Dulu ia teks telanjang yang mengambang, jadi satu
  sel dalam barisnya tampak kosong.

**Dua: pengelompokan yang salah.** Kolom setelan tadinya deretan baris tanpa
nama, dan satu-satunya judul kelompok — "Video inside the 9:16 frame" — berdiri
tepat di bawah "Social media guides" yang tidak ada hubungannya. Sekarang tiga
kelompok bernama, dan pembagiannya bisa diuji, bukan selera:

| Kelompok | Menjawab |
| --- | --- |
| **Subtitle** | rupa teksnya — font, warna, ukuran, gaya, kerapatan, garis tepi, sorotan, kotak latar |
| **Placement** | di mana ia berdiri — posisi, platform, tombol area aman |
| **Frame** | bagaimana VIDEO dipasang — pemasangan, latar, zoom |

**Pembatas platform pindah dari Frame ke Placement**, dan itu perbaikan makna
bukan pemindahan kotak: ia mengatur ke mana SUBTITLE boleh ditaruh, bukan
bagaimana video dipasang ke bingkai. Selama ia duduk di kelompok bingkai,
tombol "area aman" di sebelahnya terbaca seolah menggeser videonya.

Ikut berubah: `PLATFORMS` dan tipe `Zone` pindah dari `frame-panel.tsx` ke
`preview-panel.tsx`. Overlay zona digambar di atas bingkai pratinjau, jadi di
situlah tempatnya.

Satu kotak yang dulu muncul-hilang kini **selalu dirender dalam keadaan mati**:
warna sorotan, yang hanya berarti pada gaya menyorot. Kotak yang menghilang
membuat sel-sel di bawahnya melompat tiap gaya diganti.

### Semua label ditulis ulang jadi nama, bukan kalimat

Diminta pemilik proyek yang **tidak membaca bahasa Inggris dengan lancar** —
jadi label yang panjang bukan cuma berantakan, ia benar-benar tidak terbaca.
Aturannya satu: **label adalah NAMA (satu-dua kata), penjelasan pindah ke
tooltip `title` atau ke pilihannya sendiri.**

| Sebelum | Sesudah |
| --- | --- |
| `How the video fits` | `Fit` |
| `Center of the Picture — crop to fill` | `Crop to fill` |
| `Social media guides` | `Platform` |
| `Subtitle style` / `Subtitle pacing` | `Style` / `Pacing` |
| `Normal — whole sentences` | `Whole sentences` |
| `Output folder (optional — empty = data/<job>)` | `Output folder` (sisanya jadi placeholder) |
| `Claude model (stronger = better and more expensive)` | `Claude model` |
| `Local model (run through Ollama)` | `Ollama model` |
| `⤓ Place in the safe area` | `Move to safe area` |
| `reset to centre` | `Centre` |

Dikerjakan **di kedua kamus sekaligus** — kunci Inggris tanpa pasangan Indonesia
ditolak TypeScript, jadi tidak mungkin setengah jalan.

Satu temuan sampingan: **`cancel` berbunyi `"⏹ Batalkan"`**, dan U+23F9 itu
`Emoji_Presentation=Yes` — persis kelas lambang yang notes ini larang, dan
kebetulan tidak tertangkap grep di atas karena rentangnya tidak ikut terdaftar.
Sudah dibuang. Kalau grep-nya mau dirapikan nanti, U+23F9 dan tetangganya layak
masuk daftar.

### Kotak log memanjang sampai sejajar tombol Mulai

Diminta: "panjangkan lagi log agar bawahnya lurus dengan Start process."

Yang meregang di kolom kiri sekarang **kotak log**, bukan panel pratinjau —
kebalikan dari sebelumnya. Pratinjau setinggi isinya, sisa ruang jatuh ke log,
jadi dasar kolom kiri berhenti di garis yang sama dengan tombol Mulai di kolom
kanan (yang memang selalu menempel di dasar, sebab `SetupPanel` di atasnya
`flex: 1`).

Karena tingginya kini datang dari panelnya, `.logbox` memakai `flex: 1;
min-height: 0` — **bukan angka tinggi**. Tidak ada satu pun tinggi log yang
dipaku lagi; `min-height: 150px` pada panelnya cuma jaring pengaman untuk jendela
yang sangat pendek.

### Kartu berita disamakan dengan halaman klip (6 Agustus 2026)

Halaman `/news` adalah yang terakhir memakai bentuk lama: satu kolom sepanjang
1.127 baris yang WAJIB digulir. Sekarang bentuknya sama persis dengan halaman
klip — dua kolom, kiri yang dilihat, kanan yang disetel, kepala hanya muncul
kalau ada peringatan.

| | Kiri (`.screen-main`) | Kanan (`.screen-col`) |
| --- | --- | --- |
| isi | baris tautan artikel · pratinjau kartu (meregang) · tautan unduh | Artikel · Paragraf · Isi · Rupa, lalu tombol Simpan di dasar |

**Kotak seret foto dibuang, dan itu keputusan UI bukan penghematan.** Halaman ini
punya DUA pratinjau: kotak foto 380 px dengan CSS yang menyalin template engine,
dan pratinjau kartu sungguhnya hasil render Chrome. Keduanya tidak pernah bisa
sama persis, dan dua pratinjau yang berbeda sedikit lebih membingungkan daripada
satu yang benar. Yang tinggal: **Foto (potong/utuh) · Isian ruang kosong · Zoom**.

**Yang ikut hilang bersamanya: geseran foto manual (`offset_x`/`offset_y`).**
Itu satu-satunya kendali yang memang butuh diseret. Engine tetap menerima
medannya dan GUI mengirim 0 — kalau geseran diperlukan lagi, kembalikan bersama
kendalinya, jangan sebagai state yang selalu nol. Ini kehilangan kemampuan yang
disengaja; kalau ternyata dipakai, catatan ini tempat membatalkannya.

Ikut dirapikan sekalian:

- **Tombol "Pratinjau" dibuang.** Sudah ada auto-pratinjau berpenundaan 700 ms
  sejak lama, jadi tombolnya mengerjakan yang sudah dikerjakan sendiri. Tinggal
  satu tombol **Simpan kartu** di dasar kolom kanan — sejajar dengan tombol
  Mulai di halaman klip.
- **Semua penggeser jadi `<Stepper>`**: ukuran judul, ukuran teks, jarak isi,
  turun kartu, zoom foto. Alasan yang sama dengan halaman klip.
- **Judul + kalimat pengantar dibuang di SEMUA halaman** (`/news`,
  `/requirements`, `/clips`, `/history`) — nama halaman sudah ada di rail kiri
  dalam keadaan terpilih. Kepala hanya dirender kalau ada galat atau peringatan.
- **32 kunci i18n mati dibuang** dari kedua kamus, hasil menyisir `app/**`.

Satu keterangan yang **sengaja DIPERTAHANKAN** walau yang lain dibuang:
`creditSource` ("kreditkan sumbernya saat memposting"). Isi kartu ini verbatim
milik penerbitnya — itu kewajiban, bukan hiasan antarmuka (notes/13).

### Koreksi hari yang sama: kolomnya salah bagi

Susunan pertama menaruh **isian tautan artikel di kepala panel pratinjau** (meniru
`<SourceRow>` di halaman klip) dan **setelan rupa kartu di kolom kanan**. Dua-duanya
salah, dan pemilik proyek menunjuknya langsung: *"jangan gabungkan Article link
dengan hasil generate, pindahkan design card ke bagian preview karena itu bukan
fitur artikel."*

Kesalahannya menyalin BENTUK tanpa maknanya. Di halaman klip, path video ada di
panel pratinjau karena ia menentukan gambar apa yang muncul — sebab-akibat
langsung. Tautan artikel tidak begitu: ia mengambil TEKS, dan kartunya baru
dirender setelah puluhan setelan lain. Menempelkannya ke panel hasil menyatukan
dua hal yang tidak berhubungan.

Aturan pembagian kolomnya sekarang tertulis satu kalimat di `CLAUDE.md`:
**kiri = pratinjau + apa pun yang mengubah rupanya; kanan = masukan, pilihan,
dan tombol jalan.** Diuji begitu: "kalau kendali ini disentuh, apakah gambar di
kiri berubah?" Rupa kartu → ya, jadi ia ke kiri. Tautan artikel → tidak, ke
kanan.

Susunan `/news` yang berlaku:

| Kiri (`.screen-main`) | Kanan (`.screen-col`) |
| --- | --- |
| `.sub-layout` → pratinjau kartu + tautan unduh · **Rupa kartu** · **Foto** · **Huruf & jarak** | **Artikel** (tempel/jelajah) · **Paragraf** · **Isi** · tombol Simpan |

Kelasnya sama persis dengan halaman klip — `.sub-layout`, `.sub-preview`,
`.sub-settings`, `.grid3`, `.start-panel` — dan tinggi bingkai kartu memakai
`clamp(240px, 42dvh, 400px)` yang SAMA dengan bingkai 9:16, jadi keduanya ikut
mengecil bersama saat jendelanya pendek.

### Jelajah berita jadi popup, dan pratinjau memakai tinggi kolomnya

Dua keluhan dari tangkapan layar 6 Agustus 2026.

**1. Pencarian tidak sejajar dengan dua pintu masuk lainnya.** Kotak `.search`
berdiri sendiri di bawah tab, seolah ia setelan milik mode "jelajah". Padahal ia
**pintu masuk ketiga** — setara "tempel tautan" dan "jelajah". Sekarang
ketiganya satu baris, dan pencarian yang memakai sisa lebarnya.

Daftar beritanya sendiri pindah ke **popup yang menempel pada tombol Jelajah**,
isinya kisi **tiga kolom** yang bergulir sendiri (dua kolom di bawah 760 px).
Alasannya bukan gaya: daftar yang tinggal di dalam kolom mendorong seluruh
setelan ke bawah, dan itu berarti kolomnya bergulir.

**Popupnya `position: fixed`, BUKAN `absolute`, dan ini bukan detail.**
`.screen-col` memakai `overflow-y: auto`, dan itu MEMOTONG anak yang diposisikan
absolut begitu ia lebih lebar daripada kolomnya — kisi tiga kolom selalu lebih
lebar. Letaknya karena itu dihitung dari `getBoundingClientRect()` tombolnya dan
dipasang sebagai koordinat layar, dijepit ke lebar jendela supaya tidak pernah
keluar layar di jendela 900 px. Ditutup lewat Esc, klik di luar, atau memilih
satu artikel.

Aturan umumnya layak diingat: **di dalam kerangka berzona, semua popup harus
`fixed`.** Tiap kolom di sini punya `overflow` sendiri, jadi `absolute` akan
selalu terpotong.

**2. Panel kiri terasa kosong.** Dengan tinggi pratinjau dipaku
`clamp(240px, 42dvh, 400px)`, panel kirinya menyisakan ±500 px kotak putih
kosong di bawah setelan — dan itu terbaca sebagai halaman yang belum selesai.

Pratinjau sekarang memakai **seluruh tinggi kolomnya** (`flex: 1`), dan kolom
pratinjaunya ikut dilebarkan jadi pecahan `minmax(240px, 0.62fr)`. Sebabnya
aritmetika: kartu 9:16 setinggi 700 px butuh ±394 px lebar, jadi kolom 240 px
tetap akan menyia-nyiakan tinggi yang baru saja diberikan.

Ini **berbeda** dengan halaman klip, yang bingkainya tetap `clamp(...)` pada
kolom 240 px — dan bedanya punya alasan: di sana kolom kanan berisi kendali
subtitle yang panjang dan memang butuh lebarnya, sedangkan di sini setelan rupa
kartu cuma tiga kisi pendek. Kalau suatu saat setelan kartu ikut memanjang,
angkanya perlu ditimbang ulang.

### Enam perbaikan dari tangkapan layar (6 Agustus 2026, putaran terakhir)

**1. Rail tidak memberi tahu pengguna sedang di halaman mana.** Keadaan
terpilih memakai `background: var(--panel2)` di atas `--panel` — selisihnya
**1%**, dan itu bukan "halus", itu tidak terlihat. Sekarang latar beraksen 12%
+ garis 3 px yang **menempel ke tepi rail**, penanda posisi yang lazim di
aplikasi desktop dan terbaca tanpa dibandingkan dengan tetangganya.

Pelajarannya bisa dipakai ulang: **kalau keadaan aktif dibedakan oleh dua token
yang nilainya berdekatan, ia tidak dibedakan sama sekali.** Periksa selisih
angkanya, jangan percaya kesan saat menulis CSS-nya.

**2. Popup jadi komponen bersama** (`popover.tsx`). Dulu logikanya ditulis
langsung di halaman berita; begitu palet warna butuh popup kedua, menyalinnya
berarti menyalin juga satu detail yang mudah salah: **letaknya WAJIB `fixed`.**
Tiap kolom di kerangka ini punya `overflow-y: auto` sendiri, jadi anak yang
`absolute` akan dipotong begitu ia lebih lebar daripada kolomnya — dan isi popup
di sini hampir selalu lebih lebar. Sekali salah, dua kali salah; jadi satu
komponen.

`<Popover>` juga menutup saat **digulir** (`scroll` dengan capture), bukan cuma
Esc dan klik luar: popup `fixed` tidak ikut bergerak bersama kolom yang digulir,
jadi membiarkannya terbuka berarti ia menggantung di tempat yang salah.

**3. Warna kartu jadi popup.** Dropdown + deretan contekan yang tumbuh di bawah
kisi menggeser seluruh kelompok tiap kali mode warna diganti. Sekarang satu
tombol berisi titik warna + namanya; paletnya di dalam popup.

**4. Kelompok "Type & spacing" jadi satu baris.** Empat angka + satu tombol
tidak pernah muat satu baris, jadi kelompoknya selalu melipat jadi dua.
Tombol "kembali ke standar" **naik ke baris judul kelompok** (`.group-title
.with-action`), sisanya `.grid4`. Pola ini layak dipakai ulang: **aksi milik
kelompok tempatnya di judul kelompok, bukan sebagai sel di dalam kisinya.**

**5. Pratinjau kartu tidak lagi berlatar hitam.** Kotaknya setinggi kolom
sedangkan kartunya 9:16, jadi sisa ruang muncul sebagai dua pita hitam besar
yang terbaca sebagai "ada yang kosong". Bingkai dan bayangan sekarang menempel
pada **gambarnya**, bukan pada kotaknya — sisa ruang menyatu dengan panel, dan
ini berlaku sama untuk 4:5 dan 1:1 tanpa perhitungan tambahan.

**6. Kelompok di kolom artikel bisa diciutkan** (`section.tsx`). Keluhannya
"bagian Article masih goyang", dan sebabnya nyata: isi kolom itu muncul-hilang
mengikuti keadaan — kolom tautan hilang saat pindah ke jelajah, daftar paragraf
muncul sesudah dianalisis, seluruh blok Isi baru ada setelah artikel termuat.
Tiap kali itu terjadi, kelompok di bawahnya melompat. Menciutkan yang sudah
selesai membuat kolomnya diam.

Judulnya `<button>`, bukan `<div onClick>` — ia memang tombol, dan hanya begitu
ia terjangkau Tab dan terbaca pembaca layar.

### Tema gelap akhirnya dipasang

Struktur tokennya sudah disiapkan sejak awal justru untuk ini, dan terbukti
terbayar: **tema gelap hanya menimpa nilai `--color-*` di
`:root[data-theme="dark"]`.** Tidak ada satu pun aturan lain yang perlu tahu tema
mana yang berlaku, dan tidak ada halaman yang perlu disisir ulang.

Dua hal yang perlu diketahui sebelum menyentuhnya:

- **Nilai `-text` dibalik, dan itu disengaja.** Di tema terang, hijau/oranye/
  merah cerah tidak lolos ambang 4,5:1 di atas putih, jadi teks memakai varian
  yang digelapkan. Di atas dasar gelap masalahnya terbalik — warna cerah itu
  justru yang terbaca — jadi `--color-good-text` dsb. dikembalikan ke nilai
  cerahnya.
- **Temanya dipasang skrip kecil di `<head>`, sebelum halaman digambar.** Kalau
  dibaca dari React saja, halaman berkedip putih dulu tiap kali dibuka dalam
  tema gelap — dan kedipan itu paling terlihat di aplikasi desktop yang memuat
  ulang halamannya sendiri saat jendela ditinggal (WebView2). Bawaannya
  mengikuti `prefers-color-scheme` sampai pengguna memilih sendiri.

`.logbox` ikut dipindah ke token panggung: ia satu-satunya kotak yang hexnya
ditulis langsung, dan karena itu satu-satunya yang tidak akan ikut berganti tema.

### Chromium akhirnya ada, dan langsung membantah dua klaim saya

Seluruh catatan di atas ditutup kalimat "belum diukur, tidak ada Chrome di mesin
ini". Sejak 6 Agustus 2026 ada (`npx playwright install chromium`, tanpa root),
dan pengukuran pertama membantah dua hal yang saya tulis sendiri:

| | Klaim | Terukur |
| --- | --- | --- |
| klip di 1240×860 | "muat" | kolom kiri **+281 px**, kanan **+85 px** |
| `ⓘ` | "simbol teks biasa, aman" | **kotak kosong** — 8 buah |

`ⓘ` (U+24D8) lolos dari SEMUA grep penjaga dan sudah duduk di sana
berbulan-bulan. Pelajarannya: daftar rentang emoji tidak pernah lengkap, dan
satu-satunya pemeriksaan yang benar adalah **melihat gambarnya**.

Sebab kolom klip kelebihan tinggi ternyata aritmetika, bukan selera: kolom
setelan cuma **324 px** — di bawah ambang tiga kolom — jadi jatuh ke dua kolom
dan tingginya **697 px**. Sementara kolom pratinjau 240 px padahal bingkai 9:16
di dalamnya cuma **181 px**: 60 px menganggur, persis yang dibutuhkan
tetangganya. Sesudah pratinjau dipersempit dan pembagian kolom digeser, setelan
turun ke **474 px** tanpa satu kendali pun dibuang.

Angka sesudahnya, dan ini yang jadi baseline: **klip & kartu berita `0/0` di
1240×860.** 900×600 masih belum muat (+481 / +187) dan itu **belum dikerjakan** —
di lebar 900 kolom setelan tinggal ~180 px sehingga kendalinya wajib menumpuk.
Menutupnya butuh keputusan pemilik proyek: kendali mana yang dibuang, atau
batas minimum jendelanya dinaikkan.

Alatnya disimpan di `scripts/measure-ui.mjs`, perintahnya di `CLAUDE.md`.
**Jangan lagi menyerahkan tebakan.**

### Dua bug yang cuma ketahuan setelah dirender

**1. Penanda posisi rail mati di semua halaman kecuali `/`.** Ekspor statis Next
menghasilkan `/news/index.html`, jadi `location.pathname` berbunyi `"/news/"`
sedangkan `href` di daftar rail ditulis `"/news"`. Perbandingan `===` tidak
pernah cocok. Sekarang keduanya dinormalkan lebih dulu (`samePath`).

Ini kelas kesalahan yang layak diingat: **ekspor statis menambahkan garis miring
yang tidak ada di kode.** Cek apa pun yang membandingkan path apa adanya.

**2. Bingkai pratinjau melimpah menutupi kolom setelan** — labelnya terpotong
jadi "nt", "yle", "ghlight". Persis kegagalan yang komentar di CSS sudah
peringatkan bertahun sebelumnya, terulang karena **angkanya dua**: lebar kolom
`190px` dan tinggi `clamp(..., 400px)` ditulis terpisah, dan pada jendela tinggi
bingkainya jadi 213 px di kolom 190 px.

Sekarang satu variabel mengatur keduanya:

```css
.sub-layout { --pv-w: 190px; }
.sub-preview .preview9x16 { --pvh: clamp(200px, 42dvh, calc(var(--pv-w) * 16 / 9)); }
```

Batas atas tingginya DITURUNKAN dari lebar kolomnya, jadi ia tidak mungkin lebih
lebar daripada tempatnya berdiri. **Kalau satu ukuran harus cocok dengan ukuran
lain, jangan menulis dua angka.**

### Sapuan kode setelah semua ini

Dijalankan sekalian, dan semuanya sekarang bisa diperiksa mesin:

- `go vet` · `go build` · `go test ./...` — lolos;
- `tsc --noEmit` · `next build` — lolos;
- **konsol browser bersih di lima halaman** (galat, peringatan, dan exception
  dikumpulkan lewat CDP — sebelumnya tidak pernah bisa dilihat sama sekali);
- **13 kelas CSS mati dibuang**, disisir dengan membandingkan seluruh kelas di
  `globals.css` terhadap semua `app/**/*.tsx`: `.head-row` `.head-actions`
  (judul halaman sudah tidak ada), `.screen-body.three` (semua halaman dua atau
  satu kolom), `.pair`, `.browse-anchor` (diganti `.popover-anchor`),
  `.search-result`, `.link-row`, `.swatch-wrap`, `.source-actions`;
- satu kunci i18n mati (`newsSource`) dibuang.

### Kolom pratinjau diratakan dengan kolom setelan (6 Agustus 2026)

Keluhannya: tumpukan kendali di bawah bingkai berantakan, pratinjaunya terlalu
sempit, dan kolom mesin terlalu lebar. Ketiganya satu masalah: **kolom kiri
punya jatah tinggi tetap, dan tiap piksel yang dipakai kendali diambil dari
bingkai pratinjaunya sendiri.**

Yang dikerjakan, semuanya diukur ulang tiap langkah:

- **Muat-ulang & Bersihkan jadi tombol ikon**, dan Kisi + Garis tengah ikut satu
  bilah. Teks tombolnya yang membuat bilah itu melipat jadi empat baris di kolom
  190 px.
- **Penggeser waktu jadi satu baris** — label di atas + penggeser di bawah
  memakan 67 px; sekarang 28 px, angkanya pindah ke ujung kanan barisnya.
- **`--pv-w` 190 → 210 px**, dan angkanya diturunkan dari hasil ukur:
  bingkai (210 × 16/9) + bilah 51 + baris waktu 28 = **466 px**, yaitu tinggi
  kolom setelan di sebelahnya. Keduanya berhenti di garis yang sama.

Terukur dengan **pratinjau benar-benar termuat** (video sungguhan lewat
`localStorage`, bukan halaman kosong — dan itu perlu, sebab halaman kosong tidak
merender bilah kendali maupun penggeser waktu sama sekali):

| | sebelum | sesudah |
| --- | --- | --- |
| kolom kiri melimpah | +34 px | **0** |
| tinggi kolom pratinjau | 500 | 464 |
| tinggi kolom setelan | 466 | 466 |

### Satu percobaan yang GAGAL, dan kenapa layak dicatat

Permintaan "persempit kolom mesin" dikerjakan apa adanya lebih dulu: pembagian
`2fr : 1fr`, batas bawah 280 px. Hasilnya **kebalikannya** — kolom itu turun ke
350 px, jatuh di bawah ambang tiga kolom, dan tingginya NAIK 170 px (kelompok
Engine 280 → 357, Quality & Output 97 → 173 masing-masing).

**Mempersempit kolom berkisi bisa membuatnya lebih tinggi.** Batas bawahnya
sekarang dipaku `372px` berikut alasannya di CSS, supaya tidak ada yang
mempersempitnya lagi tanpa sadar. Yang berlaku: `1.8fr : minmax(372px, 1fr)`.

### Kesalahan paling memalukan sejauh ini: angka mati dari satu jendela

Pemilik proyek: *"wow tidak ngapa-ngapain dari tadi? tidak ada yang berubah."*
Ia benar, dan sebabnya bisa diukur.

Seluruh penyetelan tata letak sebelumnya diukur di **1240×860** lalu hasilnya
dituliskan sebagai **angka mati**: `--pv-w: 210px`. Di jendela pengguna yang
jauh lebih lebar, pratinjaunya karena itu **tetap 210 px** sementara kolom
setelan menelan seluruh ruang lebihnya. Terukur:

```
1600x1000  pratinjau 464 px
1240x860   pratinjau 464 px     <- persis sama. Nol adaptasi.
```

**Mengukur satu ukuran jendela lalu memaku hasilnya bukan mengukur — itu
menebak dengan langkah tambahan.** Sekarang tingginya diturunkan dari tinggi
jendela:

```css
--pv-h: clamp(240px, calc(100dvh - 404px), 900px);
--pv-w: calc(var(--pv-h) * 9 / 16);
```

`404px` itu jumlah semua yang BUKAN pratinjau di kolom kiri (hiasan jendela,
baris sumber, padding panel, kotak log, bilah kendali), diukur bukan dijumlah
dari angka CSS. Lebar kolom diturunkan dari tingginya, jadi bingkainya tidak
mungkin lagi melimpah keluar kolomnya. Hasilnya:

| Jendela | sebelum | sesudah |
| --- | --- | --- |
| 1240×860 | 464 | 465 |
| 1600×1000 | 464 | **605** |

Aturan yang layak dipegang: **kalau sebuah ukuran diambil dari pengukuran, ia
harus ditulis sebagai RUMUS dari sesuatu yang berubah — bukan sebagai angka.**
Angka mati hanya benar di jendela tempat ia diukur.

### `<select>` bawaan diganti komponen sendiri

Keluhan: daftar pilihannya "sangat kaku", dan *"semua pasti seperti itu"*.
Memang, dan tidak ada CSS yang bisa memperbaikinya: **daftar yang dibuka
`<select>` digambar SISTEM OPERASI, bukan halaman.** Ia tidak mewarisi satu pun
token warna kita — di tema gelap ia tetap putih menyilaukan dengan sorotan biru
bawaan Windows. Satu-satunya jalan adalah menggambar daftarnya sendiri.

`select.tsx`, dan **ke-30 `<select>` di seluruh `app/` sudah dipindah** — nol
tersisa, sebab setengah jalan berarti dua rupa berdampingan.

Yang perlu diketahui sebelum menyentuhnya:

- **Papan ketik dilayani penuh** (panah, Home/End, Enter, Esc). `<select>`
  bawaan melayaninya; menggantinya dengan yang tidak berarti membuat aplikasi
  ini lebih sulit dipakai, bukan lebih bagus.
- **Daftar membuka ke ATAS bila ruang di bawah lebih sempit**, dan boleh lebih
  lebar daripada pemicunya (280 px) — nama model tidak muat di kotak 172 px.
- **Nilai yang tidak cocok dengan satu pun pilihan menampilkan pilihan
  pertama**, persis seperti `<select>` bawaan. Tanpa ini kotak "Ollama model"
  tampil KOSONG selama efek auto-pilih belum jalan — terlihat pada potret
  pertama, dan terbaca seperti bug.
- **Keterangan pilihan di baris KEDUA, bukan berdampingan.** Sebaris,
  "8.0B · 4.9 GB · siap" mendesak nama modelnya sampai hilang sama sekali —
  juga terlihat pada potret pertama, bukan diduga.

### Kotak log, kendali pratinjau, dan tinggi sebagai JATAH

Tiga permintaan sekaligus, dan ketiganya satu persoalan: **kolom kiri punya satu
jatah tinggi, dan pratinjau, kendalinya, serta kotak log berebut jatah yang
sama.**

- **Kendali pratinjau pindah ke dasar kolom setelan.** Di bawah bingkai, tiap
  piksel yang dipakainya diambil langsung dari kotak log. Kolom setelan punya
  ruang sisa di dasarnya; bilah ini mengisinya.
- **Tinggi kendali dikembalikan ke ukuran wajar.** Keluhannya "kenapa jadi
  besar", dan memang: dengan `padding: 10px` + jarak label 6 px, tiap baris
  memakan 62 px — enam pilihan berarti 372 px hanya untuk enam kotak. Sekarang
  7 px dan 4 px. Kelompok Engine 280 → 250 px.
- **`--pv-h` berubah arti: dari "sisa ruang" jadi JATAH.** Dengan
  `100dvh - 313px` pratinjau memakan seluruh sisa dan kotak log tinggal **55 px**
  di jendela 1600×1000 — itu bukan kotak log, itu hiasan. Sekarang
  `100dvh - 380px`, menyisihkan bagian untuk log.

Terukur, dan ini yang akhirnya dilihat pengguna:

| Jendela | lebar bingkai | kotak log |
| --- | --- | --- |
| 1240×860 | 217 px | 44 px |
| 1600×1000 | **295 px** | **122 px** |

(Awalnya: bingkai 213 px, log 55 px — di kedua jendela, sebab angkanya mati.)

**Minimum dipasang pada ISI, bukan pada panel.** `min-height` pada `.log-panel`
selalu meleset: panel sudah memuat judul + padding 57 px, jadi panel 76 px
berarti kotaknya 19 px — dan 19 px memangkas baris pertamanya di tengah.
Minimumnya sekarang di `.logbox` (44 px = dua baris). **Ini jenis cacat yang
tidak muncul di angka "tidak bergulir" dan hanya ketahuan dari potret.**

### Kartu: tombol Simpan pindah ke dasar kolom setelan

Pratinjau kartu 9:16 jauh lebih tinggi daripada setelannya, jadi ada ruang
menganga ±370 px di bawah kolom setelan. Tombol **Simpan kartu** dan tautan
unduh pindah ke sana (`margin-top: auto`) — mengisi ruang itu DAN menaruh
tombolnya tepat di bawah setelan yang ia simpan. Kolom artikel tinggal berisi
artikel.

Ini menyimpang dari kerangka baku di `CLAUDE.md` (tombol aksi di dasar kolom
kanan), dan penyimpangannya disengaja: di halaman ini kolom kananlah yang tidak
punya ruang sisa, dan kolom tengah yang punya.

### Bilah atas dibuang; akun, tema, setelan pindah ke dasar rail

Isinya cuma tiga ikon, dan satu baris penuh selebar jendela untuk tiga ikon
adalah tinggi yang diambil dari isi halaman — tiap halaman, selamanya.
Ketiganya kini di `.rail-tools` di DASAR rail kiri, **berukuran sama** (32×32):
ukuran yang berbeda-beda membuat salah satunya terbaca lebih penting.

Ikut dibuang: `.topbar`, `.topbar-item`, `.settings-wrap`. Popup setelan
sekarang memakai `<Popover>` bersama, jadi tidak ada lagi dua cara memposisikan
popup di repo ini.

**Kode akses** (`account.tsx`) ditambahkan sebagai ikon ketiga. Untuk sekarang
ia HANYA MENYIMPAN, dan itu disengaja: gerbang yang benar harus diperiksa
ENGINE. Kode yang hanya dicek di JavaScript bisa dilewati siapa pun yang membuka
devtools, jadi memasangnya sekarang cuma memberi rasa aman palsu. Yang
dikerjakan sekarang tempat memasukkannya; pemeriksaannya menyusul bersama
keputusan bagaimana kodenya diterbitkan.

### Sejajar itu diukur, bukan dirumuskan

Permintaan "sejajarin aja lalu naikan log proses" tidak bisa dipenuhi dengan
rumus CSS: tinggi kolom setelan ditentukan ISINYA — berapa baris kendali yang
muat pada lebar saat itu — dan CSS tidak punya cara membacanya. Semua percobaan
sebelumnya adalah tebakan konstanta yang benar di satu ukuran jendela saja.

Sekarang `ResizeObserver` membaca tinggi kolom setelan dan menuliskannya ke
`--pv-h`. Tidak ada lingkaran umpan balik — kolom setelan sama sekali tidak
bergantung pada tinggi bingkai. Hasilnya:

| Jendela | bingkai | setelan | kotak log |
| --- | --- | --- | --- |
| 1240×860 | 479 | 479 | 71 |
| 1600×1000 | 479 | 479 | **211** |

Sejajar persis di kedua ukuran, dan seluruh sisa tinggi jatuh ke kotak log —
yang itulah yang diminta. (Sebelumnya: log 44 dan 122.)

Halaman kartu memakai cara yang sama, plus satu perbaikan: panelnya tidak lagi
meregang setinggi kolom (`flex: none`). Aturan dasar `.screen-main > .panel:
first-of-type { flex: 1 }` membuatnya meregang padahal isinya berhenti di
tengah, dan yang terlihat adalah kotak putih raksasa yang separuhnya kosong.

### Tombol Cari yang tidak melakukan apa-apa

Sejak daftar berita pindah ke `<Popover>` — yang memegang keadaan bukanya
sendiri — menekan "Cari" hanya menyetel kunci pencarian. Hasilnya tidak muncul
di mana pun. `<Popover>` sekarang bisa dikendalikan dari luar (`open` +
`onOpenChange`), jadi tombol Cari benar-benar membukanya.

Pelajarannya: **memindahkan keadaan ke dalam komponen bersama memutus siapa pun
yang dulu menyetelnya dari luar.** Cari pemanggil lamanya, jangan hanya
memastikan compiler lolos.

### Halaman kartu disusun ulang: daftar feed selalu terlihat

Bentuk lama menyembunyikan berita di balik popup "Jelajah", dan pengguna harus
memilih SATU sumber dari dropdown sebelum melihat apa pun. Sekarang:

| Kiri (`.screen-main`) | Kanan (`.screen-col`) |
| --- | --- |
| **Paragraf + Isi** jadi SATU panel di atas pratinjau · pratinjau + Rupa kartu | daftar berita dari **semua feed**, terus terlihat, dengan tombol muat ulang |

Popup jelajah, tab "Tempel tautan / Jelajah", dan pemilih feed dibuang. Kolom
tautan tetap ada untuk artikel yang TIDAK muncul di feed.

**Semua bagian bisa diciutkan** (`<Section>`), dan itu yang akhirnya membuat
halamannya muat: Foto dan Huruf & jarak tertutup secara bawaan. Terukur di
1600×1000 dengan artikel termuat: **0/0/0** — halaman, kolom kiri, kolom kanan,
tidak satu pun bergulir. Di 1240×860 kolom kiri masih 50 px lebih; menutup satu
bagian saja sudah menutupnya.

### Berita ternyata TIDAK pernah diurutkan

Diadukan pemilik proyek ("masih belum newest"), dan benar — `ListFeed`
mengeluarkan artikel **urut feed apa adanya**. Lebih buruk: `max` dipotong SAAT
MENGUMPULKAN, jadi artikel terbaru bisa ikut terbuang hanya karena feednya tidak
menaruhnya di awal.

Dua perbaikan di `engine/internal/news/rss.go`:

- `sortNewestFirst` — RFC3339 tersusun leksikografis = tersusun kronologis, jadi
  perbandingannya cukup `>`. Yang **tanpa tanggal ditaruh di BELAKANG**, bukan
  di depan: tanggal kosong berarti tidak diketahui, dan menaruh yang tidak
  diketahui di puncak daftar "terbaru" adalah kebohongan kecil yang mahal —
  orang memilih artikel dari puncak daftar. Ada testnya.
- **Batas `max` dipakai SESUDAH pengurutan**, bukan saat mengumpulkan.

Ditambah `ListAll` — merangkak semua feed bawaan **serentak** (satu feed lambat
tidak menahan sembilan lainnya), membuang duplikat berdasarkan URL (bukan judul;
judul dipoles berbeda tiap sumber), lalu mengurutkan. Feed yang gagal dilewati
diam-diam; galat hanya muncul bila TIDAK SATU PUN berhasil. Dipakai lewat
`/api/news/list?feed=all`.

### Dua cacat CSS yang hanya ketahuan dari gambar

**1. Dropdown di dalam popup = popup di dalam popup.** Pemilih warna membuka
popup, dan di dalamnya ada `<select>` yang membuka daftar KEDUA di atasnya. Yang
terlihat pengguna: dua kotak melayang bertumpuk, tidak jelas mana yang sedang
dipakai. Diganti dua tombol berdampingan (`.seg`).

**Aturannya: JANGAN menaruh dropdown di dalam popup.** Untuk dua-tiga pilihan
yang saling meniadakan, pakai tombol berdampingan.

**2. `overflow: hidden` diam-diam mengizinkan grid menggencet barisnya.** Item
grid dengan `overflow` selain `visible` kehilangan minimum otomatisnya
(`min-height` jadi 0), jadi grid berhak menyusutkannya di bawah tinggi isinya.
Terukur: 24 artikel tergencet jadi **24 px** masing-masing padahal isinya 64 px
— gambarnya terpotong dan judulnya hilang, sementara angka "tidak bergulir"
tetap hijau. Radius sudut dipindah ke gambarnya sendiri, `overflow` dibuang.

Daftar beritanya sekalian diubah jadi bentuk daftar samping: gambar kecil 64×48
di kiri, judul dua baris di kanan. Bentuk lama (gambar 16:9 selebar kolom) cuma
memuat empat artikel per layar.

### Daftar paragraf pindah ke popup; panel isi berhenti meregang

Dua keluhan yang ternyata satu sebab: **panel isi artikel mendorong pratinjau
kartu keluar layar.**

**1. Peringkat paragraf jadi popup.** Tiap paragraf itu satu alinea utuh; sepuluh
di antaranya berarti panelnya sendiri yang harus digulir, sekaligus mendorong
pratinjau turun. Sekarang ada tombol **"Pilih paragraf (N)"** di sebelah
"Analisa artikel" yang membukanya di popup selebar 760 px — di situ tiap
paragraf punya ruang untuk dibaca, dan memilih satu langsung menutupnya.

**2. Panel isi tidak lagi meregang.** Aturan dasar
`.screen-main > .panel:first-of-type { flex: 1 }` membuat panel isi memenuhi
tinggi kolom padahal isinya berhenti di tengah — kotak putih setengah kosong
YANG SEKALIGUS mendorong pratinjau ke bawah sampai kolomnya bergulir. Di halaman
kartu, semua panel kini setinggi isinya.

**3. Dua baris digabung jadi satu.** Paragraf punya empat sel di kisi TIGA
kolom (dua baris); Isi punya Sumber/Tanggal/Gambar lalu Tagar sendirian (dua
baris). Keduanya jadi `.grid4` — satu baris masing-masing.

Terukur, dengan artikel benar-benar termuat:

| | sebelum | sesudah |
| --- | --- | --- |
| tepi atas pratinjau | 553 px | **449 px** |
| kolom kiri melimpah (1240×860) | +216 | **0** |
| kolom kiri melimpah (1600×1000) | +61 | **0** |

Pelajaran yang berulang tiga kali di sesi ini: **`flex: 1` pada panel pertama
adalah aturan yang menunjuk berdasarkan URUTAN, dan tiap kali isi kolom berubah
ia menunjuk yang salah.** Halaman klip menunjuk panelnya dengan nama
(`.panel-grow`, lalu `.log-panel`); halaman kartu tidak memakai `flex` sama
sekali.

### Laporan lapangan: empat galat, dan dua di antaranya salah tuduh

**1. "Ollama is unreachable" padahal Ollama JALAN.** Pesan itu keluar untuk dua
sebab yang sama sekali berbeda — tidak bisa dihubungi, dan **menjawab terlalu
lambat** — jadi pengguna diarahkan memeriksa hal yang sudah benar
(`ollama serve`). Yang sebenarnya terjadi: model sedang dimuat ke memori dan
belum mengirim satu header pun.

`dialError` sekarang membedakannya, dan batas waktunya dinaikkan **5 → 12 menit**:
yang dihitung bukan waktu berpikir melainkan waktu MEMUAT, dan model 8B di mesin
tanpa GPU bisa perlu belasan menit pada permintaan pertama.

**Pelajarannya bisa dipakai ulang: satu kalimat galat untuk dua sebab yang beda
penanganannya lebih buruk daripada tidak ada kalimat sama sekali.**

**2. Uji LLM lokal** (`POST /api/ollama/ping`, tombol "Uji model"). "Terpasang"
tidak sama dengan "bisa dipakai" — model bisa terdaftar di `ollama list` tapi
gagal dimuat karena RAM kurang. Tombol ini menyapa modelnya dan menampilkan
balasannya. Terukur di mesin ini: **llama3.1:latest menjawab "Glowing!" dalam
7,7 detik**; model yang tidak ada memberi petunjuk `ollama pull`.

Gunanya yang kedua justru yang menentukan: **ia MEMUAT modelnya**. Sesudah uji
ini berhasil, job berikutnya tidak lagi menanggung waktu muat panjang itu
diam-diam di dalam batas waktunya.

**3. Pemilih berkas membuka path yang mustahil** (`…\bin\whisper-cli.exe`).
Sebabnya bukan di engine — `openableDir` sudah menurunkan berkas ke foldernya —
melainkan di GUI: **`onDoubleClick` memilih berkas APA PUN, mengabaikan
`pickable()` yang dipatuhi klik biasa.** Klik-ganda pada `whisper-cli.exe`
menetapkannya sebagai "video sumber", dan pemilih berikutnya dibuka dari sana.
Sekarang keduanya memakai penyaring yang sama.

**4. "Check again" — DIPERIKSA, dan ternyata bekerja.** Tes pertama saya
melaporkannya hilang; yang salah tesnya, bukan tombolnya (ia mencari teks
Inggris sementara bahasanya sedang Indonesia). Diverifikasi ulang: menekannya
memanggil `/api/requirements`. Setelan bahasa juga diuji — mengganti ke
Indonesia langsung mengubah label rail.

### Kartu: lebih besar, dan bisa dilihat penuh layar

Panel kartu sekarang memakai SISA tinggi kolomnya (`flex: 1`), bukan tinggi
setelan di sebelahnya — panel isi artikel di atasnya sudah setinggi isinya, jadi
apa pun yang tersisa memang milik kartu. Terukur: **380 → 423 px** di 1600×1000.

Diklik, kartunya terbuka **penuh layar** (857 px). Tiga jalan keluar: tombol X,
Esc, atau klik di mana pun — yang mana pun yang dicoba orang harus berhasil.

### Redundansi yang dibereskan

Diminta sekalian, dan disisir dengan alat, bukan mata:

- **`usePopup`** — `<Popover>` dan `<Select>` menulis sendiri-sendiri logika yang
  sama: hitung letak dari `getBoundingClientRect`, tutup saat Esc/klik-luar/
  gulir/ubah-ukuran. Disatukan, sebab satu aturannya mudah salah dan mahal
  (`fixed`, bukan `absolute`) — ditulis dua kali berarti dua tempat untuk salah.
- **`tsc --noUnusedLocals --noUnusedParameters`** dijalankan: nol variabel,
  impor, dan parameter yang menganggur.
- **Empat kelas CSS mati** dibuang (`.tabs`, `.tiny-select`, `.popover-head`),
  disisir dengan membandingkan seluruh kelas terhadap semua `.tsx`.
- Nol kunci i18n mati.

### Placeholder `data/<job>` diganti

`outputDirPlaceholder` berbunyi `Default: data/<job>`. Itu jargon: pengguna awam
tidak tahu apa itu `<job>`, dan tanda kurung siku terbaca seperti kesalahan
ketik. Sekarang **"Leave empty to save inside the app"** / **"Kosongkan untuk
menyimpan di dalam aplikasi"** — mengatakan hal yang sama tanpa menyebut satu
pun nama folder.

Aturan umum yang layak dipegang: **placeholder untuk pengguna, bukan untuk yang
menulis kodenya.** Nama folder sungguhan tempatnya di halaman Requirements, yang
memang menampilkan letak folder apa adanya.

Cara memeriksanya begitu ada browser, di konsol jendela aplikasi:

```js
const b = document.body, m = document.querySelector('.screen-main');
console.log('body', b.clientHeight + '/' + b.scrollHeight,
            'main', m.clientHeight + '/' + m.scrollHeight);
// body harus SAMA (halaman tidak bergulir).
// main boleh berbeda di 900x600; di 1240x860 seharusnya sama juga.
```

**Nama bagan yang dipakai sebagai bahasa bersama** (dipakai di kelas CSS, nama
komponen, DAN saat meminta perubahan) supaya tidak ada lagi salah tunjuk:

`preview` · `frame` · `render` · `source` · `ai` · `run` · `log` · `results` ·
`history` · `settings`

**Sekalian saat memecah**: judul bagan ditulis ulang jadi nama, bukan kalimat.
"How the clip looks — drag the subtitle on the preview to position it" itu
instruksi, bukan judul; jadikan **"Preview"** dan pindahkan instruksinya ke
bawahnya sebagai teks kecil (atau buang).

**Dan sekalian juga**: `globals.css` sudah 1.100+ baris padahal Tailwind sudah
terpasang. Gaya tiap komponen ikut pindah ke kelas Tailwind DI komponennya
masing-masing; `globals.css` hanya menyisakan token `@theme`, kerangka
(`.app`, `.rail`, `.topbar`, `.screen*`), dan hal yang benar-benar global.
Memindahkannya sekarang gratis — komponennya toh sedang ditulis ulang.
