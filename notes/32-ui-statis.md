# UI statis & riwayat yang bertahan — 6 Agustus 2026

Lanjutan `notes/31`. Satu syarat yang diminta berulang kali oleh pemilik proyek:
**"UI harus benar-benar statis, tidak bergerak"**. Catatan ini menulis apa yang
membuatnya bergerak, supaya tidak dipasang kembali.

## 1. Pratinjau yang "membesar sendiri" — lingkaran umpan balik

Sampai hari ini tinggi bingkai pratinjau diukur dari kolom setelan di
sebelahnya (`ResizeObserver` → `--pv-h`), sementara **lebar kolom bingkai
diturunkan dari tinggi itu** (`--pv-w: calc(var(--pv-h) * 9 / 16)`).

Itu lingkaran, dan komentar di kodenya justru menyatakan sebaliknya
("tidak ada lingkaran umpan balik"):

```
setelan lebih tinggi → bingkai melebar → kolom setelan menyempit
       → isinya melipat → lebih tinggi lagi → ...
```

Terlihat sebagai: memuat pratinjau membuat bingkainya melompat besar, dan huruf
contoh subtitle (dihitung sebagai pecahan dari `--pvh`) ikut membesar.

Sekarang ukuran bingkai **hanya bergantung pada tinggi jendela**:

```css
.sub-layout { --pv-w: clamp(150px, calc((100dvh - 300px) * 9 / 16), 506px); }
.sub-preview .preview9x16 { --pvh: calc(var(--pv-w) * 16 / 9); height: var(--pvh); }
```

Terukur di 1240×860: bingkai **262×465 pada subSize 72 DAN pada subSize 140** —
angka yang sama persis. Aturan turunannya: **jangan pernah mengukur satu kolom
untuk menentukan ukuran kolom yang mempengaruhinya kembali.** Kalau butuh
kesejajaran, ambil dari sesuatu yang tidak ikut berubah (tinggi jendela).

Pengukuran `--pv-h` di `news/page.tsx` ikut dibuang: sejak pratinjau kartu
memakai sisa tinggi kolomnya, **tidak ada satu aturan CSS pun** yang membacanya,
sementara efeknya berjalan tiap render.

## 2. `.screen-head` dibuang — notifikasi melayang

Galat, peringatan, dan bilah kemajuan dulu dirender sebagai baris kepala
halaman. Ketiganya muncul dan hilang, dan tiap kali itu terjadi **seluruh isi
halaman bergeser** — persis saat pengguna sedang menekan sesuatu.

- Galat & peringatan → `gui/app/alerts.tsx`, `position: fixed` di kanan-bawah,
  dengan **warna + lambang** (segitiga peringatan / oktagon galat) supaya
  terlihat, dan tombol tutup. Nol piksel pergeseran.
- Kemajuan job → pindah ke `<RunPanel>`, dengan tempat yang **selalu ada**
  (bar + satu baris teks, "Not running" saat diam). Tombol Batal juga selalu
  dirender, dimatikan saat tidak ada yang bisa dibatalkan.

Aturan turunannya: **apa pun yang muncul-hilang tidak boleh berada di aliran
tata letak.** Kalau ia harus di dalam panel, sediakan tempatnya secara permanen.

## 3. Popup paragraf yang menutup sendiri saat digulir

`use-popup.ts` mendengarkan `scroll` dengan **capture**, jadi gulir DI DALAM
popup ikut tertangkap dan popupnya menutup pada gerakan roda pertama — itu
"choose paragraph tidak bisa digulir". Sekarang gulir yang berasal dari dalam
popup diabaikan.

Dua tombol di tiap paragraf (`.par-actions`) kini `flex: 1`: labelnya berganti
saat dipilih ("Use on card" → "On card"), jadi tombol yang melebar mengikuti
teksnya tidak pernah sejajar dengan pasangannya.

## 4. Daftar berita tidak lagi lenyap saat memuat lebih banyak

`listBusy` mengganti SELURUH daftar dengan teks "memuat", termasuk saat gulir
tak terbatas menambah halaman. Akibatnya posisi gulir kembali ke atas tiap kali
dasar daftar tersentuh — gulir tak terbatas yang tidak bisa dipakai. Teks
"memuat" sekarang hanya muncul saat daftarnya memang masih kosong.

## 5. Kisi & garis tengah pindah ke setelan subtitle

Bilah di bawah gambar dulu memuat kisi + centang garis tengah, dan bilah itu
hanya ada saat pratinjau termuat — satu lagi sumber pergeseran. Keduanya kini
duduk di sel ketiga baris Highlight (`.field-inline`: satu Select + satu tombol
saklar), selalu ada.

## 6. Requirements MENJALANKAN ffmpeg & ffprobe

`setup.checkRuns` menjalankan `-version` dan melaporkan yang gagal sebagai
**tidak terpasang**, dengan pesan sistemnya apa adanya di `Detail`. "Ada" dan
"bisa dijalankan" adalah dua hal berbeda di Windows (arsitektur salah, DLL
kurang, dibekukan Defender), dan sampai kini keduanya dilaporkan hijau sementara
tiap job berhenti dengan galat ffmpeg — tanpa satu pun tempat yang menyebutkan
galatnya (`notes/31` butir 1).

## 7. Riwayat job bertahan — satu JSON per job

`engine/internal/job/store.go`: `<DataDir>/jobs/<id>.json`, ditulis setelah job
selesai dan setiap kali klipnya dihapus (tulis ke `.tmp` lalu rename).

- Satu berkas per job, bukan satu indeks: berkas rusak melewatkan satu job, tidak
  menjatuhkan riwayatnya.
- Saat dibaca, **klip yang berkas videonya sudah tidak ada dibuang** — pratinjau
  dan tombol unduhnya pasti gagal. Job yang seluruh klipnya hilang tetap muncul,
  dengan panel kosong.
- Job yang tercatat `running`/`queued` dibaca sebagai `canceled`: prosesnya sudah
  mati bersama aplikasi.
- Nomor urut dilanjutkan dari yang tertinggi di disk, kalau tidak job baru
  menimpa riwayat lama.

## Yang masih belum

- **900×600 masih melimpah** (klip 441 px). Belum diputuskan: buang kendali atau
  naikkan `minWidth`/`minHeight` — keputusan pemilik proyek (`notes/31` butir 3).
- **Galat ffmpeg di Windows** belum diperbaiki, hanya kini bisa dilihat. Butuh
  teks galat dari mesin itu.

## Putaran kedua — 6 Agustus 2026 (sore)

### 8. Halaman Results dibuang

Ia menampilkan job TERAKHIR saja, sementara halaman Output history menampilkan
semua job termasuk yang terakhir. Satu rail item untuk bagian dari isi halaman
lain.

### 9. Riwayat kartu: kisi yang mengalir ke bawah

Deretan mendatar cocok untuk KLIP — mereka dikelompokkan per job, jadi satu job
= satu baris. Kartu berita tidak dikelompokkan apa pun, jadi barisnya
menyisakan panel kosong selebar layar dan menyembunyikan sisanya di kanan
(terlihat pada potret, bukan pada angka). Sekarang `.card-grid`
(`auto-fill, minmax(150px, 1fr)`).

Kartu klip di deretan riwayat ikut dijinakkan: video dibatasi 240 px (dulu 9:16
selebar kartu = 391 px), judul & alasan di-clamp, hashtag disembunyikan. Satu
job kini muat di jendela: melimpah 140 → **0** di 1240×860.

### 10. Memilih: klik, Ctrl+klik, Shift+klik

Klik = pilih ini saja · Ctrl/Cmd+klik = tambah/lepas · Shift+klik = rentang.
Centang di pojok tiap item tetap ada untuk toggle tunggal. Berlaku sama untuk
klip dan kartu.

### 11. Unduh massal — satu zip

`GET /api/download?clip=<jobID>/<clipID>&card=<cardID>` (engine/internal/api/
bulk.go) mengalirkan SATU zip. Bukan sederet tautan `download`: browser bertanya
"izinkan mengunduh banyak berkas?" pada yang kedua, dan WebView2 kadang
diam-diam menolak sisanya. Semua berkas dikumpulkan sebelum satu byte terkirim —
setelah header keluar, id yang salah tidak bisa lagi dijawab 404.

### 12. Peringatan jadi LAMBANG di label (`warn.tsx`)

Baris `<div className="warn">` di bawah kendali menggeser panel setiap kali
muncul. Sekarang: segitiga 14 px di dalam label (label sudah setinggi satu baris,
jadi nol pergeseran), isinya — kalimat DAN tombol tindakannya — di popup
melayang. Dipakai untuk: subtitle menabrak zona, API key kosong, Ollama mati,
model belum diunduh/tidak mampu, koreksi transkrip tanpa LLM, ping gagal.

Bedanya dengan `alerts.tsx`: Warn menempel pada kendali yang bersalah, Alerts
untuk galat setingkat halaman (engine tak terjangkau, komponen kurang).

### 13. Popup tombol rail keluar ke SAMPING

`usePopup({ side: "beside" })`: dasar popup sejajar dasar tombolnya (CSS
`translateY(-100%)`, sebab tingginya belum diketahui saat posisinya dihitung),
keluar ke kanan rail. Sebelumnya popup Settings membuka ke atas setinggi 460 px
dan menutupi separuh halaman, terbaca lepas dari tombol yang membukanya.

### 14. Tombol di kisi: satu baris, tinggi sama

`.grid4 > .field-check` sekarang ikut `align-self: stretch` + rata dasar seperti
`.grid3` (dulu hanya kisi 3 kolom, dan baris "Paragraph" di tab kartu jadi
satu-satunya yang tombolnya melayang di ketinggian label). Tombol di kisi
`white-space: nowrap`, dan LABELNYA yang dipendekkan ("Analyse article" →
"Analyse", "Choose paragraph" → "Paragraphs", "Test the model" → "Test") —
tombol yang membungkus dua baris berdiri lebih tinggi daripada tetangganya.

Tombol uji model pindah ke kolom ketiga kisi Engine, sejajar dengan dua Select
di kirinya; hasilnya tidak lagi menambah baris (berhasil → tombol hijau +
tooltip, gagal → lambang peringatan di label model).

### 15. Tinggi kendali: satu variabel (`--ctl-h`)

Terukur di baris "Paragraph": `<Select>` **top 83, tinggi 32**; tombol di sel
sebelahnya **top 75, tinggi 40** — 8 px lebih tinggi DAN 8 px lebih ke atas,
karena tombol kisi memakai angkanya sendiri (`min-height: 37px` + padding).
Sekarang `--ctl-h: 32px` dipakai `<input>`, `.sel`, dan tombol di kisi; keempat
kendali baris itu **top 83, tinggi 32**.

Label yang jadi dua baris ikut dipendekkan ("Move to safe area" → "Safe area"):
dengan tinggi terkunci, label panjang meluap, bukan menaikkan tombol.

### 16. Sisa peringatan yang masih menggeser — semuanya dibereskan

Setelah butir 12, enam tempat lain masih memakai pola lama:

| Tempat | Dulu | Sekarang |
| --- | --- | --- |
| Kartu: catatan mesin skor | `.warnbox` di bawah kisi | `<Warn>` di sel tombol |
| Kartu: "menganalisis (lokal)…" | baris `.stage` | tooltip pada tombolnya |
| Kartu: "hasil pencarian untuk X" | baris tambahan di atas daftar | tombol Search berganti jadi Clear |
| Requirements: galat pemasangan | baris `⚠` di bawah keterangan | `<Warn>` di nama komponen |
| Requirements: kemajuan unduhan | bar + pesan, dua baris baru | bar `position:absolute` di dasar baris; keterangan berganti teks (satu baris, elipsis) |
| Klip: status font manual | baris yang panjangnya berubah tiap ketikan | satu baris tetap + `<Warn>` di label |

Yang TIDAK diubah: `⚠` di dalam kotak log. Log memang daftar yang bergulir —
itu satu-satunya tempat teks boleh bertambah.
