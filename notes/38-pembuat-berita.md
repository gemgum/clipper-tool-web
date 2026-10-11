# Pembuat Berita — Rencana & Keputusan Desain

Tanggal diskusi: 17 Agustus 2026. **Belum ada kode.** Berkas ini merekam
keputusan yang sudah diambil supaya sesi berikutnya tidak mengulang diskusinya.

Fitur: ambil sampai 5 artikel berita → LLM menulis ulang jadi satu artikel baru
(judul + badan) → keluar sebagai berkas siap salin-tempel, dengan gambar utama
dari artikel sumber.

Sasarannya **media berita milik pemilik proyek sendiri**, bukan konten media
sosial. Itu yang membedakannya dari tab kartu berita.

## Ini permukaan KETIGA, bukan pengembangan tab kartu

Aturan keras tab kartu (`notes/13`, diulang di CLAUDE.md): **LLM hanya memilih
nomor paragraf, tidak pernah menulis** — isi kartu & caption selalu verbatim
dari artikel.

Fitur ini kebalikannya: LLM menulis. Kalau keduanya dijadikan satu tab, cepat
atau lambat teks karangan LLM keluar sebagai kutipan verbatim. Karena itu:

| | Kartu berita | Pembuat berita |
| --- | --- | --- |
| Pembaca | jempol di HP | pembaca situs sendiri |
| Keluaran | PNG 1080×1920 | judul + badan artikel + gambar |
| Aturan teks | verbatim | LLM menulis, dijaga pagar fakta |
| Folder data | `data/cards` | `data/posts` |

Paket `card` TIDAK dipakai di sini — tidak ada teks yang dibakar ke gambar.

## Cara memasukkan artikel sumber: tiga jalan, satu keranjang

Diputuskan 17 Agustus 2026. Ketiganya harus ada, dan boleh dicampur dalam satu
job (mis. 2 dari pencarian + 3 dari tempel URL):

| Jalan | Dipakai saat | Fondasi yang sudah ada |
| --- | --- | --- |
| Cari kata kunci | tahu topiknya, belum tahu artikelnya | `news/search.go` + Google News RSS |
| Jelajah & klik beberapa | menelusuri daftar berita | `news/rss.go`, sama seperti tab kartu |
| Tempel URL | sudah punya alamatnya | `news/article.go` |

Bedanya dengan tab kartu: di sana satu artikel sekali jalan, di sini daftar
berita bisa **dicentang beberapa** (maksimal 5) dan tempel URL menerima
beberapa alamat sekaligus (satu per baris).

Apa pun jalannya, hasilnya masuk ke **satu keranjang sumber** yang sama —
itu yang dibaca pipeline, bukan asal-usulnya. Konsekuensi yang wajib dikerjakan:

- **Dedup lewat URL yang sudah di-resolve**, bukan URL mentah. Artikel yang sama
  bisa masuk dua kali lewat jalan berbeda: sekali sebagai pengalih
  `news.google.com/rss/articles/...` dari daftar, sekali sebagai alamat asli
  yang ditempel. `news/resolve.go` sudah membongkar pengalih itu — dedup terjadi
  SESUDAH resolve.
- Batas 5 berlaku di keranjang, bukan per jalan.
- Artikel yang gagal diambil isinya dibuang dari keranjang dengan alasannya,
  tidak diam-diam dihitung sebagai sumber kosong.

## Alur: dua tahap, dan itu bukan pilihan gaya

`news/select.go` memakai `maxPromptWords = 1200`, dipatok untuk satu artikel di
konteks 8k model lokal. Lima artikel ≈ 4.000 kata ≈ 6.000 token, belum termasuk
prompt dan ruang menulis. Dikirim sekaligus berarti terpotong — dan gejalanya
persis yang harus dihindari: model jatuh ke judul karena badannya hilang.

```
Tahap 1 (per artikel, 5 panggilan)  badan artikel PENUH → daftar fakta bernomor
                                    judul SENGAJA tidak dikirim
Tahap 2 (satu panggilan)            5 daftar fakta → judul + badan artikel
```

Judul dibuang di tahap 1 supaya model tidak punya jalan pintas: satu-satunya
bahan yang ada cuma badan artikelnya.

`maxPromptWords` yang konstan sebaiknya diganti perhitungan dari `NumCtx`
(`ollama.Client` sudah membaca konteks asli model dari metadata Ollama).

## Tahap 1: SUDAH DIKERJAKAN (17 Agustus 2026)

`engine/internal/writer/facts.go` — `ExtractFacts()`. Diuji terhadap llama3.1
lewat Ollama, artikel tiruan dan artikel Antara sungguhan.

```
CLIPPER_TEST_LIVE=1 go test ./internal/writer/ -run Live -v
CLIPPER_TEST_LIVE=1 CLIPPER_TEST_URL=<url berita> go test ./internal/writer/ -run Live -v
```

Hasil pada artikel Antara (12 paragraf, 287 kata): 12 fakta, tersebar dari
paragraf 0 sampai 10, nol ditolak, ~20–30 detik termasuk memuat model. llama3.1
sanggup mengerjakan tahap ini.

Tiga hal yang baru ketahuan setelah diuji, dan sudah dikerjakan:

**1. Nomor paragraf dari model tidak boleh dipercaya.** Pada artikel nyata,
llama3.1 salah menyebut nomor pada 2 dari 12 fakta — meleset satu. Satu meleset
tertangkap pemeriksaan "berbagi kata", satu lagi LOLOS karena paragraf sebelahnya
kebetulan memuat istilah yang sama ("IRGC").

Ini fatal kalau dibiarkan: SELURUH pagar tahap 2 memverifikasi terhadap nomor
ini. Salah nomor = memverifikasi ke sumber yang keliru, dan hasilnya "lolos".

Jadi `resolve()` mencari sendiri paragraf asal sebuah fakta lewat jumlah kata isi
yang sama; nomor dari model cuma pemecah seri. Yang dikoreksi ditandai `recited`
supaya mutu model bisa diukur, bukan disembunyikan.

**2. Ambang dua kata (`minOverlap`).** Satu kata sama terlalu murah — kalimat
karangan seperti "Presiden meninjau lokasi bencana" menempel ke paragraf mana pun
yang memuat "bencana". Dua kata menutup itu, dan fakta sungguhan hampir selalu
membawa lebih.

**3. Fakta negatif harus diminta eksplisit.** Percobaan pertama melewatkan
"tidak berpotensi tsunami" dan "tidak ada korban jiwa" — di berita justru sering
fakta terpentingnya. Satu baris tambahan di prompt menyelesaikannya.

Yang belum: cache hasil per artikel (pola `news/select.go`), dan jatah kata yang
dihitung dari `NumCtx` alih-alih `DefaultMaxWords` yang tetap.

## Tahap 2: SUDAH DIKERJAKAN (17 Agustus 2026)

`engine/internal/writer/compose.go` — `Compose()`. Beberapa lembar fakta → satu
artikel, lalu pagar `inspect()`.

```
CLIPPER_TEST_LIVE=1 go test ./internal/writer/ -run ComposeLive -v
```

Kebijakan perbaikannya seperti diputuskan: melanggar → kirim balik SEKALI dengan
daftar pelanggarannya → draf kedua dipakai HANYA bila pelanggarannya lebih
sedikit (perbaikan tidak boleh membuat lebih buruk) → artikel tetap keluar apa
pun hasilnya, dengan pelanggaran ikut di keluaran.

Tiga hal yang baru ketahuan setelah diuji ke llama3.1:

**1. Model mengabaikan sumber kedua — dan itu tidak kelihatan.** Draf pertama
menyalin ulang sumber pertama saja; sumber kedua (Sanden, Bupati, BPBD DIY)
hilang seluruhnya. Artikelnya tetap enak dibaca dan tetap bebas fakta karangan,
jadi tanpa pemeriksaan khusus kegagalan ini lolos diam-diam — padahal lima
artikel yang menyusut jadi satu berarti seluruh gunanya fitur ini hilang.

Dua perbaikan, dan keduanya perlu:

- **Pagar `coverage`**: sumber yang kata khasnya tidak muncul di artikel
  dilaporkan. "Khas" = kata yang tidak ada di artikel sumber lain mana pun.
- **Fakta diselang-seling antar sumber** di prompt, bukan dikelompokkan per
  sumber. Dengan blok per sumber, model membaca blok pertama lalu berhenti.

**2. Selang-seling melahirkan masalah baru: penanda `[sumber/paragraf]` ikut
tersalin ke badan artikel.** Akibatnya bukan cuma jelek dibaca — nomor sumber
terbaca pagar sebagai ANGKA KARANGAN, sehingga satu artikel bersih dilaporkan
melanggar enam kali. Dibersihkan deterministik oleh engine (`Draft.strip`),
karena bentuk penandanya pasti; permintaan di prompt cuma pelengkap.

**3. Pagar nama diri sempat melaporkan SEMUA nama sebagai karangan.**
`normalize()` mengapit hasilnya dengan spasi supaya frasa bisa dicari utuh, dan
lookup per kata memakai hasil itu apa adanya — jadi tidak pernah cocok. Ketahuan
dari uji unit, bukan dari mata. Pagar yang menandai setiap nama sah sama tidak
bergunanya dengan tidak ada pagar.

Setelah ketiganya: tiga jalan berturut-turut bersih — nol pelanggaran angka,
kutipan, nama, cakupan, dan peta klaim. Yang tersisa cuma `length`, dan itu
memang benar: dua artikel tiruan yang pendek tidak menyediakan 400 kata bahan.

**Catatan tentang llama3.1 untuk tahap 2:** sanggup, tapi hanya setelah dua
perbaikan sisi engine di atas. Peta `claims` kadang diisi kadang tidak — karena
itu ia tidak diwajibkan di skema. Lembar fakta tahap 1 tetap jadi jejak audit
utama; `claims` bonus. Kalau nanti mutunya kurang, tahap 2 adalah SATU panggilan
dengan masukan padat — paling murah dipindah ke Claude/Gemini tanpa memindahkan
tahap 1.

## Tahap 3: SUDAH DIKERJAKAN (17 Agustus 2026)

`engine/internal/writer/output.go` — `Save()`. Rantainya kini utuh dan teruji
ujung-ke-ujung lewat `TestComposeLive`, yang ikut menulis berkasnya.

```
data/posts/2026-08-17_11-04_gempa-magnitudo-5-2-guncang-bantul/
├── artikel.md    blok peringatan (bila ada) → # judul → lead → badan → atribusi
├── sumber.json   artikel sumber + lembar fakta + peta klaim + pelanggaran
└── gambar.jpg    og:image sumber pertama yang punya gambar
```

Keputusan yang mengunci di sini:

- **Blok peringatan ditulis DI DALAM `artikel.md`**, di paling atas. Inilah yang
  membuat kebijakan "pagar tidak pernah menggagalkan job" aman: peringatannya
  ikut terbawa saat berkasnya disalin. Boleh dihapus pemilik — tapi harus
  sengaja.
- **Gambar gagal TIDAK menggagalkan penyimpanan.** Artikelnya jauh lebih
  berharga daripada gambarnya; alasannya dicatat di `Post.ImageNote`.
- **Teks tetap mengikuti `lang`** (`Dirangkum dari:` / `Summarised from:`),
  seperti berkas pendamping paket `card`. Bawaan `en`.
- Nama folder `tanggal_jam_slug`; bentrok dalam menit yang sama diberi akhiran
  angka, tidak menimpa.

**Temuan uji ujung-ke-ujung: model memadatkan dengan mengulang dirinya.**
Diminta 400 kata padahal bahannya cuma ~120 (dua artikel tiruan), llama3.1
menyalin lead jadi paragraf pertama dan menggandakan paragraf. Panjangnya
"tercapai" tanpa satu pun fakta baru — dan tanpa pemeriksaan, itu terbaca
seperti artikel yang wajar. Ditambahkan pagar `repetition` (ambang 80% kata isi
yang sama).

Pelajaran yang lebih besarnya: **target 400–700 kata hanya masuk akal kalau
bahannya cukup.** Dengan 5 artikel sungguhan bahannya ada; dengan 2 artikel
pendek, pelanggaran `length` dan `repetition` justru laporan yang benar.

## Keranjang & orkestrasi: SUDAH DIKERJAKAN (17 Agustus 2026)

`gather.go` (`Gather`) dan `run.go` (`Run`), plus `summary.go` untuk ringkasan
waktu per tahap.

**`Gather`** menerima daftar alamat dari jalan mana pun — ketiga cara masuk
bermuara ke satu daftar, jadi engine tidak perlu tahu asal-usulnya. Yang
dikerjakan: ambil isi, dedup SESUDAH resolve, batas 5 di keranjang, laporkan
yang gagal beserta alasannya, tandai artikel yang topiknya jauh.

**`Run`** menjalankan tahap 1–3 berurutan dengan callback kemajuan berbentuk
sama seperti pipeline klip, supaya lapisan job & SSE memperlakukan keduanya
sama. Tahapnya: `gathering → reading → writing → saving → done`.

**Biaya satu job = (jumlah artikel + 1) panggilan LLM**, plus satu lagi kalau
pagar memicu perbaikan. Ada test yang menjaga angka itu: kalau naik, biayanya
naik untuk tiap job selamanya.

Ringkasan waktu per tahap ikut ditulis di akhir job (tabel monospace), sama
alasannya dengan pipeline klip: yang mau dibandingkan antar percobaan adalah
mesin LLM-nya — 6 panggilan ke model lokal versus 6 ke Claude bukan cuma beda
harga, tapi beda menit.

**Catatan yang mengejutkan di uji:** nama media di atribusi bisa muncul sebagai
domain, bukan nama medianya. Itu perilaku `news.siteBadge` yang memang sengaja:
`og:site_name` gampang dipalsukan, jadi ia hanya dipercaya kalau cocok dengan
domainnya. Tetap dibiarkan — domain yang benar lebih baik daripada nama yang
mengaku-aku. Tautan lengkapnya ada di `sumber.json`.

## API & CLI: SUDAH DIKERJAKAN (17 Agustus 2026)

**CLI:** `clipper write <url>... [flags]`. Ada supaya fitur ini bisa dipakai dan
diukur sebelum tab GUI-nya dibuat, dan supaya ada satu jalan yang tidak
bergantung pada browser sama sekali.

**API:** `POST /api/posts`, `GET /api/posts`, `GET /api/posts/events` (SSE),
`GET /api/posts/{id}`, `GET /api/posts/{id}/file?name=article|sources|image`.

Job disimpan di `internal/api/posts.go` mengikuti pola **installs.go**, BUKAN
`job.Manager`. Sebabnya `job.Job` berbentuk klip (`Clips`, `config.Options`,
riwayat di disk) dan menggenerikkannya berarti membongkar paket yang sudah
jalan. Hasil kerja pembuat berita sudah ada di folder artikelnya sendiri, jadi
daftar job cukup jadi jendela ke yang sedang berjalan.

Dua hal diekspor supaya api dan CLI memilih dengan cara yang PERSIS sama:
`api.LLMEngine` (perakit mesin LLM) dan `api.Browser`.

### Tiga kegagalan yang cuma ketahuan dari uji lapangan

Diuji dengan tiga artikel Antara tentang Paskibraka, lewat CLI, model llama3.1.

**1. Peringatan "topik jauh" salah tuduh — ketiganya.** `offTopic` membandingkan
JUDUL saja, dan ketiga judul cuma berbagi satu kata ("paskibraka") — di bawah
ambang dua. Judul terlalu sedikit bahannya; sekarang paragraf pertama ikut
dibandingkan.

**2. Balasan model terpotong di tengah JSON, job gagal total.** Artikel 700 kata
BESERTA peta klaimnya melewati 4096 token. Jatahnya dinaikkan ke 8192, dan
pesan galatnya sekarang membedakan balasan yang TERPOTONG dari yang bentuknya
salah — keduanya muncul sebagai "unexpected end of JSON input", padahal yang
satu butuh jatah token dan yang satu butuh model lain.

**3. Ekor artikel berisi salinan.** llama3.1 menulis sembilan paragraf bagus
lalu mengejar target 400 kata dengan menyalin paragraf 6–9 menjadi 10–16 —
sebelas pelanggaran `repetition` dalam satu artikel. Sekarang paragraf yang
menyalin **dibuang**, bukan cuma dilaporkan: membuangnya operasi mekanis tanpa
penilaian, dan draf yang tujuh paragraf terakhirnya salinan hampir tidak berguna
buat redaktur. Yang dibuang tetap tercatat di `Violations`.

### Angka dari jalan yang bersih

```
Gather sources       0.1s     0%  3 articles
Read Antara News    42.7s    20%  18 facts, 0 rejected
Read Antara News    38.4s    18%  19 facts, 0 rejected
Read Antara News    19.4s     9%   9 facts, 0 rejected
Write article       1m53s    53%  267 words, repaired once
Save files           0.1s     0%
TOTAL               3m34s
3 source articles, 1 unverified item
```

Artikelnya nyata dan layak: menggabungkan ketiga sumber, nol fakta karangan,
gambar terunduh. Satu-satunya pelanggaran `length` — dan itu benar, llama3.1
memang berhenti di 267 kata.

**Vonis llama3.1 untuk seluruh rantai:** sanggup, tapi tahap 2 memakan >50%
waktu dan mutunya paling goyah di sana. Kalau nanti kurang, tahap 2 adalah SATU
panggilan dengan masukan padat — paling murah dipindah ke Claude/Gemini tanpa
memindahkan tahap 1 yang lima panggilan.

## Tab GUI: SUDAH DIKERJAKAN (17 Agustus 2026)

`gui/app/writer/page.tsx` — tab `/writer`, ikon pena, di antara Kartu berita dan
Riwayat.

Bentuknya menyalin halaman `/` apa adanya, sesuai aturan CLAUDE.md:

```
.screen-body.two
├── .screen-main   KIRI, yang dilihat: panel Artikel (+ blok peringatan, foto,
│                  judul, badan) lalu kotak log
└── .screen-col    KANAN, yang diisi: Keranjang, Artikel sumber (tempel URL +
                   cari + daftar), Mesin AI, tombol Mulai
```

Ketiga jalan masuk ada di satu panel: tempel tautan (beberapa sekaligus, satu
per baris), cari kata kunci, dan klik `+ Tambah` di daftar berita. Semuanya
bermuara ke keranjang yang sama, dengan batas 5 terbaca di judulnya sejak
sebelum tombol ditekan.

**Terukur, bukan ditebak** (`scripts/measure-ui.mjs`):

```
writer  light  1240x860   765/765   main over 0   col over 0
writer  dark   1240x860   765/765   main over 0   col over 0
```

Dua penyetelan tata letak lahir dari MELIHAT potretnya, bukan dari angkanya:

- Kotak log semula memakan seluruh kolom kiri (bawaannya begitu, dan itu benar
  di halaman klip yang panel pertamanya bingkai pratinjau). Di sini tokoh
  utamanya artikel; log dibatasi jadi satu strip.
- Foto dibatasi 130px: pada 200px, JUDUL artikel terdorong ke bawah lipatan
  kotak — padahal itu hal pertama yang perlu terbaca begitu job selesai.

Jalur API-nya ikut diuji langsung: `POST /api/posts` → SSE `stage` bergerak
`reading → writing → done` → ketiga berkas terlayani (`artikel.md` 200
text/markdown, `sumber.json` 200 application/json, `gambar.jpg` 200 177 KB).

## Pagar fakta

Pemeriksa deterministik yang jalan SETELAH LLM menulis. Kode biasa yang
mencocokkan teks — bukan LLM lain. Pola yang sama sudah terbukti di paket
`correct` (`notes/14`): empat pagar, koreksi ditolak dilaporkan, bukan diulang
selamanya.

Yang diperiksa:

| Pagar | Isi |
| --- | --- |
| Angka | tiap angka di keluaran harus ada di paragraf yang ditunjuk sebagai sumber |
| Kutipan | teks dalam tanda kutip harus PERSIS sama dengan sumbernya |
| Nama diri | nama yang tak muncul di satu pun artikel sumber → ditandai |
| Panjang | badan tidak melebihi batas; lead 1–2 kalimat |
| Atribusi | ditempel engine, bukan LLM |

Balasan LLM di tahap 2 berskema, dan wajib memuat
`klaim[] → {teks, artikel_ke, paragraf_ke}`. Field itu yang membuat pemeriksaan
mungkin — sekaligus **membuktikan model membaca badan artikel**: yang cuma
membaca judul tidak bisa menyebut nomor paragraf yang lolos periksa.

Yang pagar TIDAK lakukan: menilai kebenaran beritanya (kalau ketiga sumber
sama-sama salah, pagar diam), menilai mutu tulisan, atau mengerti makna. Dia
menjamin satu hal saja — tidak ada fakta baru yang muncul entah dari mana.

### Kebijakan saat pagar menolak: JANGAN pernah menggagalkan job

Diputuskan 17 Agustus 2026. Berbeda dari kebijakan mesin skor (`notes/12`) yang
menghentikan job saat gagal — di sini job **selalu menghasilkan artikel**.

1. Pelanggaran ditemukan → kirim balik ke LLM sekali, sebutkan klaimnya.
2. Masih melanggar → artikel tetap ditulis ke disk, pelanggarannya ditandai.

Alasannya: keluarannya draf, bukan terbitan. Selalu ada mata manusia sebelum
terbit, jadi menggagalkan job cuma memaksa mengulang manual untuk pelanggaran
yang mungkin sepele.

**Konsekuensinya harus dijaga:** peringatan wajib ikut terbawa saat berkasnya
disalin, bukan cuma muncul di GUI yang bisa terlewat. Jadi daftar klaim tak
terverifikasi ditulis di dalam `artikel.md` sendiri, di bagian paling atas.
Pemilik boleh menghapusnya — tapi harus sengaja, bukan karena tak pernah tahu.

## Keluaran

Belum perlu integrasi CMS. Satu folder per artikel, mengikuti pola job klip:

```
data/posts/2026-08-17_11-04_prabowo-anggaran/
├── artikel.md      judul (baris pertama) + badan; blok peringatan di atas bila ada
├── gambar.jpg      og:image dari artikel sumber
└── sumber.json     artikel sumber + peta klaim → artikel/paragraf
```

`.md` dipilih karena bisa dibuka di mana saja DAN sudah siap kalau nanti mau
ditempel ke WordPress/Ghost — "belum perlu integrasi web" hari ini tidak
menutup pintu besok.

`sumber.json` itu alat redaksi, bukan metadata: saat memeriksa sebelum terbit,
tiap klaim bisa dilacak asalnya tanpa membaca ulang 5 artikel.

Di GUI: pratinjau + tombol salin judul, salin artikel, buka folder.

## Mesin LLM

`news.Completer` (`func(ctx, system, user) (string, error)`) sudah jadi colokan
yang dibutuhkan. Menambah penyedia = menambah satu adaptor tipis, bukan
integrasi baru:

| Penyedia | Teks | Gambar | Kerja |
| --- | --- | --- | --- |
| Claude (`score/llm`) | ya | **tidak punya** | nol — `Complete` sudah cocok |
| Ollama / lokal | ya | tidak | sudah ada |
| Gemini | ya | ya | pakai `Endpoint` di `score/ollama/openai.go` |
| ChatGPT | ya | ya | sama, ganti base + key |

**Pemilih mesinnya harus DUA** — mesin teks dan mesin gambar, terpisah. Kalau
dijadikan satu, memilih Claude membuat fitur gambar mati diam-diam.

Kebijakan `notes/12` tetap: mesin yang dipilih dipakai apa adanya, gagal ya
berhenti dengan pesan akar masalah. Tanpa perpindahan diam-diam.

## Gambar: og:image sumber, ilustrasi AI DITUNDA

Gambar utama diambil dari `og:image` artikel sumber (sudah diambil
`article.go`) dengan kredit media aslinya. Nol komponen baru, dan untuk media
berita justru yang paling benar: foto asli peristiwa asli.

Ilustrasi AI ditunda, dengan alasan terukur — mesin pemilik proyek adalah
**GTX 1660 Ti, 6 GB VRAM**, dan `llama3.1:8b` (4,9 GB) sudah menghuni hampir
seluruhnya:

- LLM teks tidak bisa membuat gambar sama sekali; Ollama tidak punya fitur itu.
- Membuat gambar di lokal butuh tumpukan lain (Stable Diffusion/ComfyUI).
  SDXL butuh ~8–10 GB. SD 1.5 muat tapi mutunya kelas 2022.
- GTX seri 16xx punya masalah fp16 yang dikenal di Stable Diffusion (hasil
  hitam) — harus fp32: dua kali VRAM, separuh kecepatan.
- Bobot multi-GB berarti komponen baru di `internal/setup` lengkap dengan
  sha256 (aturan CLAUDE.md), dan itu harus dirawat.

Kalau nanti benar dibutuhkan, cloud (Gemini) beberapa sen per gambar jauh lebih
murah daripada memaksa tumpukan lokal di GPU ini.

**Kalau ilustrasi AI dikerjakan nanti, dua pagar ini ikut:**

1. Foto sumber TIDAK dikirim sebagai rujukan piksel — itu membuat karya turunan
   dari foto ANTARA/AFP/Reuters. Yang dikirim: ringkasan teks hasil model vision
   (suasana, warna dominan), lalu model gambar bekerja dari teks saja.
2. Gaya dipaku di kode ke non-foto (vektor datar / kolase editorial), label
   "Ilustrasi" dan kredit "Dibuat AI" ditempel engine. Gambar AI fotorealistik
   atas peristiwa nyata adalah risiko kredibilitas terbesar sebuah media.

Yang BISA dikerjakan LLM lokal terkait gambar hari ini: model vision kecil
(~1–2 GB, muat berdampingan) untuk menulis caption/alt-text dari foto sumber
dan menandai kalau fotonya tidak nyambung dengan isi artikel.

## Tabel keputusan

| Hal | Keputusan |
| --- | --- |
| Maksimal artikel sumber | 5, dihitung di keranjang |
| Cara memasukkan sumber | tiga jalan: cari, jelajah & centang beberapa, tempel URL |
| Dedup sumber | lewat URL yang sudah di-resolve, bukan URL mentah |
| Baca isi, bukan judul | dua tahap; judul tidak dikirim di tahap 1; dibuktikan pagar |
| Gambar | `og:image` sumber + kredit; ilustrasi AI ditunda |
| Keluaran | folder per artikel: `artikel.md` + gambar + `sumber.json` |
| Pagar ditolak | perbaiki sekali, lalu **tetap keluarkan artikel** dengan tanda |
| Integrasi CMS | tidak sekarang; `.md` menjaga pintunya tetap terbuka |
| Mesin LLM | pemilih teks & gambar TERPISAH |
| Tab | **baru: `/writer`** |
| Topik yang tidak nyambung | **ditandai, bukan dilarang** — irisan kata kunci judul |
| Panjang badan artikel | **400–700 kata** |
| Paket Go | **`internal/writer`** |

Empat keputusan terakhir diambil 17 Agustus 2026, alasannya:

- **Tab `/writer` sendiri**, bukan menyusup ke `/news`: aturan verbatim tab
  kartu tidak boleh bersinggungan dengan teks karangan LLM. Harganya daftar RSS
  dipakai di dua tempat — dibayar sadar. Aturan tampilan CLAUDE.md tetap
  berlaku: jendela tidak boleh bergulir, halaman `/` adalah standar bentuknya.
- **Topik jauh ditandai, bukan ditolak.** Lima artikel yang topiknya berbeda
  (3 kecelakaan + 2 anggaran) menghasilkan artikel kacau, tapi pemiliknya yang
  paling tahu apakah kaitannya memang ada. Peringatan sebelum job jalan, bukan
  larangan.
- **400–700 kata** karena pagar memeriksanya, jadi harus ada angka. Panjang
  berita daring umumnya. Bisa dijadikan setelan GUI kalau ternyata mengganggu.
- **Paket `internal/writer`**, bukan menambah ke `news` yang sudah 2.000-an
  baris dan perannya jelas berbeda. `news` tetap dipakai bersama untuk RSS,
  pencarian, resolve, dan ekstraksi artikel.

## Dua temuan lapangan (17–18 Agustus 2026)

Job lokal dua artikel Antara (llama3.1 kedua tahap) keluar sebagai artikel
**dua kata**. Fakta tahap 1 baik-baik saja — 20 dan 19 fakta, nol ditolak — jadi
seluruh kerusakannya di tahap 2.

**1. Pagar `repetition` menghabiskan artikelnya sendiri.** Badannya tiga
paragraf: paragraf 1 menyalin lead, sisanya sampah (`"claims"`, `"%20[{"`).
`dropRepeats` membuang paragraf 1 — mekanis, benar menurut aturannya — dan yang
tersisa dua kata. Pembuangan sekarang **bersyarat**: kalau yang tersisa separuh
kata atau kurang, pengulangannya cuma dilaporkan dan draf dibiarkan utuh.
Kalimat aturannya: pagar boleh melaporkan, tidak boleh membuat lebih buruk. Draf
bertele-tele masih bisa dipangkas redaktur; artikel dua kata tidak bisa apa-apa.

**2. "Antara 400 dan 700 kata" bukan perintah yang bisa diikuti model kecil.**
Itu angka yang tak bisa dilacak sambil menulis, dan hasilnya artikel satu
paragraf — keluhan pemilik proyek "sangat pendek sekali hasilnya" berakar di
sini, bukan di jumlah faktanya. Sasarannya sekarang disebut sebagai **jumlah
paragraf**: `paragraphTarget` menerjemahkan jumlah fakta jadi 4–10 paragraf
(≈3 fakta per paragraf), dan tiap paragraf diminta 60–100 kata. Batas total
tetap ada karena pagar `length` memeriksanya.

Ekornya: `minItems: 3` pada skema `body` DIPAKSAKAN Ollama lewat grammar. Model
yang kehabisan bahan tetap wajib mengeluarkan tiga string, dan ia mengisinya
dengan sampah — dari situ `"claims"` dan `"%20[{"` datang. Skema yang dipaksakan
server tidak menjamin isi, cuma bentuk; karena itu promptnya kini menyebut
tegas bahwa memenuhi hitungan dengan paragraf kosong lebih buruk daripada
artikel pendek.

## Jendela konteks: keluaran besar terpotong diam-diam (18 Agustus 2026)

Gejalanya: `unexpected end of JSON input — the reply was cut off`, muncul begitu
artikelnya diminta lebih panjang (5 sumber, puluhan fakta).

Sebabnya bukan prompt. Engine meminta `num_predict = 8192` **di dalam**
`num_ctx = 8192`. Promptnya sendiri sudah memakan ribuan token dari jendela yang
sama, jadi model berhenti di dinding konteks di tengah JSON — dan Ollama TIDAK
melaporkannya sebagai galat, ia mengembalikan potongan itu sebagai balasan
normal. Selama ini tidak kelihatan sebab semua pemakai lama (pemilihan momen,
koreksi transkrip) balasannya pendek.

Perbaikannya di `ollama.ctxFor(promptChars, numPredict)`: jendela diminta
sebesar `numPredict + prompt/3 + 256`, tidak pernah turun di bawah 8192, dan
tidak pernah melebihi kemampuan model. Batas terakhir itu wajib — meminta 16k
pada model yang cuma sanggup 4k membuat Ollama membalas KOSONG, kegagalan yang
lebih sulit dilacak daripada yang sedang diperbaiki.

Syaratnya `Client.NumCtx` terisi. Di jalur klip itu sudah dilakukan
`resolveOllama`, tapi `api.EngineFor` membuat kliennya sendiri dan tidak pernah
mengisinya — jadi jendelanya terkunci di bawaan. Sekarang diisi lewat
`ollama.ContextOf`, sekali per job, bukan per panggilan.

## Memilih draf terbaik: menghitung pelanggaran memenangkan draf terkosong

Temuan terpenting hari itu, dan yang paling lama tersembunyi.

Pass kedua dipakai bila `len(second.Violations) < len(draft.Violations)`. Kedengaran
netral. Padahal artikel **35 kata** hanya melanggar dua hal (pendek, cakupan),
sementara artikel **500 kata** yang menyebut enam nama diri melanggar tujuh. Jadi
tiap kali perbaikan berhasil — lebih panjang, lebih banyak fakta terpakai —
hasilnya justru DIBUANG. Makin ketat pagarnya, makin pendek yang keluar. Itulah
sebabnya keluhan "sangat pendek sekali" tidak pernah hilang walau promptnya
berkali-kali diperbaiki: yang salah bukan penulisannya, melainkan penilaiannya.

`better(a, b)` sekarang menilai **kekurangan kata lebih dulu**, baru jumlah
pelanggaran. Artikel yang panjangnya masuk akal selalu menang atas yang
kependekan; di antara dua yang sama-sama kependekan, yang lebih panjang menang.

Angka satu artikel yang sama (2 sumber Antara, llama3.1 kedua tahap):

| Keadaan                              | Kata | Paragraf | Pelanggaran        |
| ------------------------------------ | ---- | -------- | ------------------ |
| sebelum semuanya                     |    2 |        1 | 5                  |
| + sasaran paragraf                   |  147 |       10 | 2                  |
| + "3–5 kalimat" (tanpa `better`)     |   35 |        3 | 2 — makin buruk    |
| + `better`                           |  372 |        9 | 11 (9-nya nama EN) |
| + bahasa disebut tegas + `bulletRe`  |  333 |       10 | **1** (panjang)    |

Dua ekor yang ikut ketahuan di baris keempat: model MENERJEMAHKAN artikel Antara
ke bahasa Inggris (aturan "write in the same language as the facts" tidak
cukup — bahasanya kini disebut tegas lewat `languageOf`, penghitung kata fungsi
sepuluh baris), dan tiap paragraf diawali `- ` sehingga artikel.md keluar sebagai
daftar berpoin (`bulletRe` di `strip`).

**Perlu keputusan pemilik proyek:** `MinWords = 400` tetap, tidak peduli
bahannya berapa. Dua artikel pendek cuma memberi bahan untuk ±330 kata, jadi
menuntut 400 justru mengundang tambalan — persis yang dijaga pagar `repetition`.
Pilihannya: (a) biarkan, "pendek" cuma tanda buat redaktur; (b) batas bawahnya
ikut jumlah fakta.

## Tagar, tautan sumber, centang keranjang (18 Agustus 2026)

Tiga permintaan pemilik proyek, dan satu bug lama yang ketahuan saat mengerjakannya.

**Tagar** ditulis model di panggilan tahap 2 yang sama (field `tags`), lalu
disaring engine dengan penyaring yang SUDAH ADA di tab kartu berita
(`news.Content.Hashtags`): kata kunci yang tidak muncul di artikelnya sendiri
dibuang. Tanpa itu tagar jadi satu-satunya celah mengarang yang tersisa — ia
terlalu pendek untuk ditangkap pagar angka, kutipan, atau nama, tapi cukup untuk
menempelkan artikel ke peristiwa yang tidak ada di dalamnya.

**Tautan sumber** ditempel ENGINE di kaki `artikel.md` (`sourceLinks`), judul +
alamat + nama media, satu baris per sumber. Bukan sekadar nama media seperti
sebelumnya: berkas ini yang ditempel ke media pemilik proyek, dan atribusi tanpa
tautan tidak bisa diperiksa pembaca. Ikut juga saat "Copy the article" ditekan,
sebab atribusi yang harus diingat sendiri cepat atau lambat lupa ditempel.
`Result.Sources` menyediakan bentuk ringkasnya untuk GUI — `Basket.Sources`
membawa artikel penuh dan memang tidak dikirim ke JSON.

**Centang hijau** di daftar berita begitu artikel masuk keranjang. `opacity: .5`
dari `button:disabled` harus ditimpa: tombolnya mati karena tidak perlu ditekan
lagi, bukan karena tidak boleh, dan tombol pudar membacanya persis terbalik.

**Bug lama yang ketahuan:** keranjang berisi 5 sumber mendorong panel "Start
processing" **188 px keluar jendela**. Sudah begitu sejak tab ini dibuat; tidak
pernah terukur sebab `measure-ui.mjs` memotret halaman dengan keranjang KOSONG.
Keranjang kini duduk di dalam panel sumber (hemat 69 px judul+padding+jarak, dan
tempatnya memang di situ — isinya persis apa yang baru dicentang dari daftar di
bawahnya), tiap barisnya satu baris berelipsis, dan daftarnya dipatok 106 px.

Dua jebakan pengukuran dari sesi itu, keduanya patut diingat:

- `min-height: 0` pada panel sumber membuat angka "kolom tidak bergulir" tetap
  **0** sementara panelnya menyusut di BAWAH tinggi isinya — kotak tempel, kotak
  cari, dan seluruh daftar berita hilang di balik panel mesin AI. Angkanya
  bersih, potretnya tidak. Lantainya wajib ada.
- Lantai itu harus disempitkan ke panel yang IKUT memuat keranjang
  (`:has(.basket)`). Dipasang ke semua `.feed-panel`, tab kartu berita ikut
  membayar tingginya: 31 → 89 di 900×600.

Dengan "mesin tahap tulis" dinyalakan kolomnya memang tidak muat dan bergulir
153 px. Disengaja: bergulir dengan segala sesuatu terlihat lebih baik daripada
muat dengan setengah panel tersembunyi. Kalau mau 0 juga di keadaan itu, yang
harus dipangkas adalah tinggi `EnginePicker` (dua tombol Test masing-masing
memakan satu baris sendiri), bukan lantai panel sumber.

## Kaki artikel sumber lolos jadi FAKTA

`junkPhrases` sudah memuat "copyright" dan "hak cipta", tapi Antara menulis
pemberitahuan lisensinya sebagai kalimat larangan penuh tanpa satu pun kata itu:

> Dilarang keras mengambil konten, melakukan crawling atau pengindeksan otomatis
> untuk AI di situs web ini tanpa izin tertulis dari Kantor Berita ANTARA.

Ia lolos penyaring, dibaca tahap 1 sebagai fakta yang sah (memang ada di
paragraf!), dan keluar di artikel jadi sebagai "Dilarang keras mengambil konten,
tetapi informasi tentang Paskibraka dapat diperoleh dari sumber-sumber resmi."
Pagar tidak bisa menangkapnya — kalimat itu BENAR ada di sumbernya.

Frasa larangan lisensi ditambahkan ke daftar, plus "pewarta:" (label baris nama
penulis). Yang sengaja TIDAK dipakai: "dilarang keras" sendirian, sebab berita
sungguhan memakainya ("polisi menyatakan dilarang keras…"); yang dipakai selalu
menyertakan objeknya ("dilarang keras mengambil"). Ujinya memuat kedua sisi:
tiga kalimat boilerplate harus terbuang, dua kalimat berita biasa harus lolos.

Tidak ada cache isi artikel — `FetchContent` mem-parsing tiap kali — jadi
perubahan ini langsung berlaku, tanpa kunci versi seperti `transcriptCacheKey`.

## Klik barisnya, bukan tombolnya

Tombol "+ Add" tersendiri dibuang. Mengklik berita di daftar = memasukkannya ke
keranjang, mengkliknya lagi mengeluarkannya — seluruh baris tidak punya arti
lain di tab ini, jadi menyempitkan sasaran klik jadi seukuran tulisan "Add"
hanya membuat pekerjaan lebih sulit. Sama persis dengan tab kartu berita.

Tombol kecil di baris itu sekarang **salin tautan**, yang memang tidak bisa
ditebak dari mengklik apa pun. Logikanya diangkat ke `gui/app/copy-link.tsx` dan
dipakai kedua tab: isinya dua jebakan yang gampang salah kalau disalin —
pengalih news.google.com harus diresolusi dulu, dan `navigator.clipboard` TIDAK
ADA (bukan sekadar gagal) saat GUI dibuka lewat alamat IP mesin.

Centang hijau tetap ada sebagai PENANDA, bukan tombol.

## Catatan jujur

Dengan fitur ini Clipper jadi dua produk dalam satu aplikasi: pemotong video dan
alat redaksi. Masih masuk akal karena ~60% fondasinya sudah ada dan dipakai
bersama (RSS, ekstraksi artikel, Chrome, job+SSE, klien LLM) — tapi lebih baik
disadari sekarang daripada saat rilis.
