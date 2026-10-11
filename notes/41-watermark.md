# 41 — Watermark: gambar PNG milik pengguna + headline

Dikerjakan 6 September 2026. Diminta untuk **ikut kontes clipper**: tiap
postingan harus membawa identitas akun yang sama.

Sempat saya beri nama "Branding" — dan itu keliru, ditegur pemilik proyek pada
hari yang sama: *branding* adalah TUJUANNYA, bukan nama bendanya. Namanya
**Watermark**, dan itu berlaku sampai ke nama paket, flag CLI, rute API, dan
kelas CSS-nya — nama yang beda antara yang dilihat pengguna dan yang tertulis di
kode adalah cara paling pelan untuk kehilangan keduanya.

## Bentuknya: dua lapis, keduanya digeser seperti subtitle

1. **Gambar** — PNG buatan pengguna sendiri. Teksnya boleh sudah ikut di dalamnya.
2. **Headline** — teks di atasnya. Diketik sendiri, atau (di halaman klip)
   diambil dari judul yang sudah dipilihkan LLM untuk klip itu.

## Yang TIDAK dibuat, dan kenapa

**Jalur render ketiga.** Keduanya menumpang jalur yang sudah ada:

| Lapis | Jalur | Berkas |
| --- | --- | --- |
| gambar | satu rantai `movie=…[blogo];[bvid][blogo]overlay=…` di dalam `-vf` | `ffmpeg.watermarkChain` |
| headline | satu Style + satu baris `Dialogue` di .ass yang sama dengan subtitle | `subtitle.writeHeadline` |

Karena burn subtitle adalah filter **terakhir** di rantai, teks otomatis
mendarat di atas gambar: tidak ada satu baris pun kode pengurutan lapis.

**`-filter_complex`.** Banner dipasang lewat sumber `movie=`, bukan masukan
kedua. Alasannya bukan selera: `ReframeFilter` dipakai bersama oleh render klip
DAN pratinjau satu frame (`/api/frame`), dan keduanya menyerahkan satu string
filter. Beralih ke `filter_complex` berarti memecah lagi jalur yang sengaja
disatukan — dan pratinjau yang punya kode sendiri adalah pratinjau yang cepat
atau lambat berbeda dari hasilnya.

**Renderer teks baru (`drawtext`, atau paket `card`).** `card` merender kartu
1080x1920 penuh lewat Chrome headless; memakainya di sini berarti menyeret
Chrome jadi syarat render klip padahal `.ass` sudah ada, lengkap dengan font,
warna, garis tepi, dan waktu tayang.

## Ukurannya KOTAK, bukan satu angka

Diperbaiki di hari yang sama setelah dicoba. Tiga laporan — "gambar masih
mengikut ukuran", "tidak bisa dipindah, cuma di center", "memotong" — ternyata
**satu baris CSS**:

```css
.preview9x16 img { height: 100%; object-fit: cover; pointer-events: none; }
```

Kekhususannya (0,1,1) menang atas `.wmoverlay` (0,1,0), jadi aturan untuk FRAME
VIDEO ikut menelan overlay watermark: tingginya dipaksa setinggi bingkai,
`cover` memotongnya, dan `pointer-events:none` membuatnya mustahil diseret.
Sekarang `.preview9x16 img:not(.wmoverlay)`.

Sekalian modelnya diperbaiki:

- **Dua angka, Width & Height**, keduanya persen sisi bingkai. Satu angka
  memaksa pengguna menghitung sendiri tinggi yang akan muncul dari rasio
  gambarnya.
- Keduanya menyatakan **KOTAK**, dan gambarnya dimuat UTUH ke dalamnya —
  `force_original_aspect_ratio=decrease` di ffmpeg, `object-fit: contain` di
  pratinjau. Bukan `scale=w:h` mentah (menggepengkan logo orang) dan bukan crop
  (memotongnya).
- **Bawaannya 25% x 25%**, bukan 92%. Watermark yang menutupi separuh layar
  bukan identitas melainkan gangguan, dan bawaan yang hampir penuh membuat orang
  mengira ukurannya memang tidak bisa diatur.
- **Headline berhenti mengikuti lebar gambar.** Dulu ia dipenggal selebar
  banner, karena gambarnya diasumsikan kartu selebar layar dan teksnya duduk di
  dalamnya. Dengan kotak seperempat bingkai, asumsi itu menghasilkan empat
  karakter per baris. Sekarang acuannya lebar bingkai: gambar dan headline dua
  lapis yang berdiri sendiri.

## Tiga laporan berikutnya

**"Tidak ada penanda grid tengah?"** — betul, dan akibatnya lebih besar daripada
kerapian: tanpa kisi dan garis tengah, tidak ada satu pun tanda bahwa gambar dan
teks di atas bingkai BISA dipegang. Pertanyaan berikutnya di pesan yang sama —
"teksnya tidak bisa dipindah?" — terjawab dengan pengukuran: bisa, sejak awal.
Yang hilang penandanya, bukan fungsinya.

Kisi, garis tengah, sumbu, kendali kerapatan, dan kotak angka posisi kini satu
komponen bersama (`gui/app/guides.tsx`) yang dipakai kedua halaman. Ditambah
kotak **posisi headline** sendiri: menyeret memberi rasa, angka memberi yang
bisa diulang — dan tanpa kotak itu tidak ada tanda bahwa teksnya bisa dipindah
terpisah dari gambarnya.

**Daftar pilihan menggantung jauh di atas tombolnya.** Bukan bug halaman
watermark — bug `usePopup` sejak awal, terukur `gap: 183 px` di halaman klip
juga. Popup yang membuka ke atas diposisikan `top = r.top - maxH`, padahal
`maxH` itu tinggi MAKSIMUM (280) sedangkan daftar berisi tiga pilihan cuma
103 px; selisihnya jadi jarak kosong. Daftar panjang kebetulan setinggi maxH,
jadi bug ini tidak pernah terlihat di sana.

Tingginya baru ada setelah dirender, jadi tidak bisa dihitung di JavaScript.
Sekarang dasar popup dipatok ke tepi atas tombol lewat kelas `up`
(`translateY(-100%)`) — mekanisme yang sudah dipakai mode "beside". Sesudahnya
`gap: 6 px` di kedua halaman.

**Kotak "Headline" abu yang tidak bisa disentuh.** Di halaman watermark
sumbernya selalu teks sendiri, dan selnya sempat DIMATIKAN, bukan dibuang.
Pertanyaan pertama yang datang: "ini kenapa tidak bisa dimasukkan?" — tepat.
Catatan ini sudah memuat aturannya satu bagian di atas ("pilihan yang selalu
gagal adalah pilihan palsu") dan tetap dilanggar dalam bentuk lain: kendali
yang ada tapi tidak bisa apa-apa. Sekarang selnya tidak dirender sama sekali,
dan `hlSource` dipaksa `"text"` saat memulihkan setelan — supaya nilai
selundupan dari versi lain tidak mematikan kotak teksnya tanpa ada kendali
untuk menghidupkannya lagi.

Diperiksa lewat CDP, bukan dengan melihat: tidak boleh ada `button.sel:disabled`
atau `input:disabled` di kelompok Watermark. Halaman watermark `dead: []`.
(Tombol −/+ Stepper yang mati di angka batasnya tidak dihitung — itu batas,
bukan kendali mati.)

**Koordinat empat angka terpotong** ("1080 · 19…"). Titik pemisahnya dirapatkan
jadi `1080·1920`; terukur `over: 0`. Angka yang terpotong tidak lebih berguna
daripada tidak ada angka sama sekali.

## Keputusan yang diambil

- **Burn saja.** Berkas `clean` tetap bersih: ia dipakai untuk disunting ulang
  di editor lain, dan identitas yang sudah terbakar tidak bisa dilepas lagi.
- **Waktu tampil relatif AWAL KLIP.** `-ss` diletakkan sebelum `-i` di
  `ClipReframe`, jadi `t` di `enable` memang dihitung dari awal keluaran.
  `enable` hanya ditulis kalau waktunya benar-benar diatur — bawaannya gambar
  tampil sepanjang klip, sebab syarat kontes lazimnya "identitas harus
  terlihat", bukan "berkedip".
- **Koordinat 1080x1920, sama dengan subtitle.** libass menskalakan sendiri
  lewat PlayRes; `overlay` tidak punya PlayRes, jadi `watermarkChain` yang
  menskalakannya. Itu yang membuat angka di pratinjau = angka di hasil.
- **Menggeser gambar ikut membawa headline.** Keduanya tetap koordinat mutlak;
  "teks relatif terhadap gambar" terdengar lebih rapi tapi menambah satu ruang
  koordinat lagi yang harus diterjemahkan di tiga tempat. Yang dibutuhkan cuma
  satu perilaku, dan itu selisih dua angka.
- **Pemenggalan baris digandakan** — `subtitle.headlineLines` (Go) dan
  `wrapHeadline` (gui/app/watermark-model.ts). Sengaja: engine yang menulis .ass, tapi
  pratinjau harus menunjukkan pemenggalan yang SAMA. Angkanya (margin 60,
  faktor lebar huruf 0,6) harus berubah bersamaan.
- **Font headline = font subtitle.** Satu pemilih font, satu metrik yang sudah
  dihitung engine (`fontmetrics.go`), satu hasil. Pemilih kedua berarti
  pengukuran kedua, dan pratinjau yang meleset dari render sudah pernah terjadi
  (notes/29).
- **Sumber "llm" hanya di halaman klip.** Halaman watermark tidak punya klip,
  jadi pilihannya dibuang dari daftarnya — bukan dibiarkan ada lalu ditolak
  engine. Pilihan yang selalu gagal adalah pilihan palsu.
- **Video yang bukan 9:16 DITOLAK, per berkas.** Reframe otomatis berarti
  memotong video orang tanpa diminta — perubahan yang jauh lebih besar daripada
  yang ia minta. Galatnya menyebut ukuran aslinya, dan berkas lain tetap jalan.
- **Sumber tidak pernah ditimpa**: hasilnya `<nama>_watermarked.mp4`.

## Tinggi: dua kali melanggar, dua kali diperbaiki

Aturan "jendela tidak boleh bergulir" jebol dua kali di sini, dan keduanya
ketahuan dari `scripts/measure-ui.mjs`, bukan dari membaca CSS:

1. Kelompok Watermark **ditambahkan** ke kolom setelan halaman klip → kolom kiri
   +43 px di 1240x860 (dan +268 px saat kelompoknya dibuka). Perbaikannya:
   kelompok ini **menukar** isi kolom — selagi terbuka, kelompok subtitle,
   penempatan, dan bingkai disembunyikan — plus jarak antar-kelompok dirapatkan.
2. Tab keenam membuat **rail** 516 px di jendela 900x600 yang cuma punya 505 px,
   dan yang bergulir SELURUH HALAMAN. Perbaikannya: padding `.rail-item`
   10 → 8 px dan jarak ikon-label 5 → 4 px, mengembalikan 30 px.

Kalau tab ketujuh ditambahkan, ukur lagi.

## Bukti

- `ffmpeg.TestClipReframeWithWatermarkAndSubtitles` — benar-benar memanggil ffmpeg
  dengan rantai `movie=` DAN `subtitles=` sekaligus. Sambungan itu yang paling
  rawan dan tidak bisa dibuktikan tes string.
- `dragtest` lewat CDP: `pointer-events: auto`, `object-fit: contain`, kotak
  65x116 px (bukan setinggi bingkai), dan seretan memindahkan 540·640 → 700·880
  dengan headline ikut serta.
- `watermark.TestRunRejectsWrongAspectButKeepsGoing` — 16:9 ditolak, 9:16 di
  sebelahnya tetap jadi.
- `subtitle.TestWriteASSAddsHeadline` / `…StaysPlain` — style kedua muncul saat
  ada teks, dan tidak muncul saat tidak ada.
- measure-ui: `clips` dan `watermark` keduanya `0/0` di 1240x860.
