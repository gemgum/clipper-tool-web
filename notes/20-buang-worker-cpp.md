# Memindahkan worker C++ ke Go

**SELESAI 4 Agustus 2026.** Keenam langkah dikerjakan. Bukti kecocokannya: Go
dan C++ diadu pada WAV 38 menit (73,7 MB, 36.868.087 sampel) dan menghasilkan
**23.043 jendela RMS yang sama persis** — 23.042 jendela penuh + satu jendela
sisa 887 sampel, cocok dalam batas 1e-5 (presisi `%.5f` yang dipancarkan C++).

Jebakan yang benar-benar muncul saat pengerjaan, semuanya sudah diperingatkan di
bagian "Hal yang gampang bikin salah" di bawah:

- Berkas WAV keluaran ffmpeg ternyata **memang** menyisipkan chunk `LIST`
  (`INFO ISFT Lavf60.100`, 23 byte — panjang ganjil, jadi ada byte sisipan).
  Data suara mulai di **byte 78, bukan 44**. Ada test yang mengunci ini:
  WAV dengan dan tanpa `LIST` wajib menghasilkan angka identik.
- Sampel dibaca tak-bertanda dua kali berturut-turut (`int64(Uint16(…))` lalu
  `uint16(Uint16(…))`). Keduanya menghasilkan angka yang sama persis dan tidak
  pernah error — ketahuan hanya karena diadu dengan C++.

Catatan kerja di bawah ini disimpan apa adanya sebagai rekaman rencananya.

---

Catatan kerja untuk menghapus `worker/` dan memindahkan tugasnya ke Go.
Ditulis untuk dikerjakan sendiri — tidak ada kode di sini, hanya peta jalannya.

## Kenapa

Worker C++ cuma mengerjakan satu hal: menghitung energi suara (RMS) dari berkas
WAV. Itu pekerjaan yang Go bisa lakukan sendiri tanpa pustaka tambahan.

Yang jadi masalah bukan kodenya, tapi ongkosnya. Selama masih ada satu berkas
C++, setiap sistem operasi yang mau kita tuju butuh compiler C++ sendiri. Untuk
Windows dan macOS itu artinya cross-compile — bagian paling merepotkan dari
seluruh rencana pengemasan.

Begitu C++ hilang, membangun untuk tiga OS jadi satu perintah `go build`.

Ada satu fakta yang membuatnya makin masuk akal: RMS hanya dipakai mesin skor
heuristik. Kalau kamu memakai Ollama atau Claude, angka itu dihitung lalu
dibuang. Jadi selama ini kita memelihara satu bahasa untuk kode yang jarang
terpakai.

## Yang sebenarnya dikerjakan worker sekarang

Buka `worker/main.cpp`. Isinya cuma dua perintah:

- `features` — yang benar-benar dipakai. Baca WAV, hitung RMS tiap 100 ms.
- `reframe` — placeholder. Langsung membalas error, belum pernah dipakai.

Jadi yang perlu kamu pindahkan cuma `features`.

Menariknya, dari 167 baris itu sekitar separuhnya bukan logika sama sekali —
itu parser JSON tulisan tangan dan pemancar NDJSON. Semuanya ada karena worker
adalah proses terpisah yang harus bicara lewat stdin/stdout. Begitu jadi fungsi
Go biasa, seluruh bagian itu lenyap tanpa diganti apa pun.

Perkiraan hasil akhir: sekitar 60 baris Go menggantikan 167 baris C++ ditambah
96 baris klien subprocess di `engine/internal/worker/worker.go`.

## Struktur folder

Sebelum:

```
worker/
  main.cpp
  CMakeLists.txt
  build/
engine/internal/worker/
  worker.go            ← memanggil biner lewat subprocess
bin/
  clipper-worker       ← hasil kompilasi
```

Sesudah:

```
engine/internal/audio/
  features.go          ← hitung RMS langsung di Go
  features_test.go
```

Folder `worker/`, paket `engine/internal/worker/`, dan biner
`bin/clipper-worker` semuanya hilang.

Nama paket `audio` cuma usulan. Yang penting namanya jujur — paket lama
bernama `worker` karena memang memanggil worker; kalau isinya jadi perhitungan
langsung, nama itu jadi menyesatkan.

## Langkah-langkahnya

Urutannya penting. Jangan menghapus dulu.

**1. Bangun worker C++ sekali lagi.** Pastikan `bin/clipper-worker` ada dan
jalan. Ini akan jadi pembanding nanti, dan begitu sumbernya dihapus kamu tidak
bisa membuatnya lagi.

**2. Tulis paket Go-nya.** Cukup satu fungsi: terima path WAV dan jarak hop
dalam milidetik, kembalikan deretan angka RMS. Bentuk hasilnya (struct
`FeaturesResult`) sebaiknya dipertahankan apa adanya — dengan begitu
`heuristicSelect` dan `rmsSlice` di pipeline tidak perlu disentuh sama sekali.

**3. Bandingkan dengan C++.** Jalankan keduanya pada WAV yang sama, cocokkan
angkanya. Ini langkah yang paling gampang dilewati dan paling menyesal kalau
dilewati.

**4. Sambungkan ke pipeline.** Ada lima titik di `pipeline.go` yang menyebut
paket `worker`. Ganti semuanya ke paket baru.

**5. Baru hapus.** Setelah semua hijau: hapus `worker/`, hapus
`engine/internal/worker/`, hapus `bin/clipper-worker`.

**6. Rapikan skrip build.** `build.sh` dan `setup.sh` masing-masing punya blok
cmake untuk worker. Buang.

## Berkas yang disentuh

| Berkas | Yang berubah |
| --- | --- |
| `engine/internal/audio/features.go` | baru — isi logikanya |
| `engine/internal/pipeline/pipeline.go` | 5 titik: import, field struct, konstruktor, pemanggilan, dua tanda tangan fungsi |
| `engine/internal/config/config.go` | `Paths.Worker` tidak dipakai lagi |
| `build.sh` | buang blok cmake |
| `setup.sh` | buang tahap worker (3/4) |
| `CLAUDE.md` | peta arsitektur & tabel paket |
| `notes/01-arsitektur.md` | diagram tiga lapis ikut berubah |

Satu hal yang ikut hilang: pemeriksaan `Available()`. Dulu perlu, karena biner
bisa saja belum dibangun. Sekarang kodenya menyatu — tidak ada yang bisa hilang.

## Hal yang gampang bikin salah

Ini bagian yang paling layak kamu baca pelan-pelan.

**Header WAV bukan selalu 44 byte.** Banyak tutorial mengajarkan "lewati 44
byte lalu baca sampel". Itu sering benar, tapi tidak selalu — ffmpeg kadang
menyisipkan chunk metadata sebelum data suara. Kalau kamu asal melompat 44
byte, metadata itu ikut terbaca sebagai suara. Telusuri chunk satu per satu dan
cari yang bernama `fmt ` dan `data`. Perhatikan `fmt ` pakai spasi di akhir.

**Chunk disejajarkan ke angka genap.** Kalau ukuran sebuah chunk ganjil, ada
satu byte sisipan sesudahnya. Lupa ini bikin chunk berikutnya terbaca meleset
satu byte, dan semuanya berantakan sesudah itu.

**Ukuran di header bisa berbohong.** Kalau rekaman terpotong, angka panjang
data di header lebih besar daripada isi berkas sebenarnya. Batasi dengan ukuran
berkas asli, jangan percaya begitu saja.

**Angkanya little-endian dan bertanda.** Sampel 16-bit itu bilangan bertanda
(-32768 sampai 32767). Kalau salah baca sebagai tak-bertanda, separuh gelombang
suaranya jadi kacau — dan hasilnya tetap "terlihat wajar", jadi bug ini sulit
terlihat tanpa dibandingkan.

**Jangan muat seluruh berkas ke memori.** WAV dari video 38 menit itu sekitar
73 MB, dan dari video 4 jam jauh lebih besar. Baca bertahap per blok. Ini juga
sejalan dengan prinsip yang sudah dipakai di tempat lain di proyek ini.

**Jumlah angka hasilnya harus sama.** RMS dipetakan ke waktu lewat `hop_ms` di
`rmsSlice`. Kalau jumlah elemennya bergeser satu saja, seluruh penilaian
heuristik ikut bergeser. Ini yang paling penting dicocokkan saat membandingkan
dengan C++.

## Cara memastikan hasilnya benar

Bandingkan keluaran Go dan C++ pada WAV yang sama. Yang perlu kamu lihat:

- jumlah angkanya **harus** sama persis
- nilainya boleh beda tipis di belakang koma — itu wajar, karena urutan
  penjumlahan pecahan berbeda
- kalau bedanya besar atau polanya bergeser, biasanya penyebabnya salah satu
  dari daftar jebakan di atas

WAV siap pakai ada di folder job lama, di `data/*/tmp/audio.wav`.

Tulis juga test kecil dengan WAV buatan sendiri — misalnya nada tetap yang
sudah kamu tahu berapa RMS-nya. Itu menangkap kesalahan yang tidak kelihatan
saat membandingkan dua berkas panjang.

## Kalau nanti butuh C++ lagi

Rencana face-follow dengan OpenCV memang butuh C++. Menghapus worker sekarang
tidak menutup jalan itu — kontrak stdin/NDJSON-nya sudah tercatat di
`notes/01-arsitektur.md` dan di komentar `main.cpp` yang bisa kamu lihat lagi
lewat riwayat git.

Menahan sebuah bahasa "untuk jaga-jaga" itu biaya yang dibayar setiap hari demi
manfaat yang belum tentu datang. Kalau nanti benar-benar dikerjakan, worker bisa
dihidupkan lagi — dan saat itu ia punya alasan yang nyata untuk ada.
