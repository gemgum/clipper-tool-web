# Pesan untuk pengguna, bukan untuk pengembang — 6 Agustus 2026

Dari pengujian v0.6.0 di Windows. Tiga hal berbeda yang tampak seperti satu:

## 1. "Run ./setup.sh base then reload"

`./setup.sh` adalah skrip repo. Ia TIDAK ikut dipasang bersama aplikasi, dan di
Windows bahkan tidak bisa dijalankan. Menyuruh pengguna menjalankannya sama
dengan menyuruhnya berhenti.

Diganti di tiga tempat — peringatan model di halaman klip, `transcribe.Available`
(biner & model whisper), dan hint whisper di `setup/recipe.go` — semuanya kini
menunjuk **halaman Requirements dan tombol Install**, yang memang mengerjakannya.
Pesan "engine versi lama? jalankan ./bin/clipper serve" ikut diganti jadi "tutup
Clipper lalu buka lagi".

Aturannya: **pesan yang dilihat pengguna hanya boleh menyebut hal yang ada di
dalam aplikasi.** Perintah terminal boleh tinggal di log, tidak di notifikasi.

## 2. "Harus di-restart dulu" — dua daftar yang tidak pernah diperbarui

- **Popup Settings** menyimpan hasil `/api/requirements` yang PERTAMA
  (`if (!open || items) return`), jadi daftar yang terbaca sebelum komponen
  dipasang tetap berbunyi "missing" selamanya — sementara halaman Requirements
  di belakangnya sudah hijau semua. Sekarang ditarik ulang setiap kali dibuka.
- **Daftar model whisper di halaman klip** ditarik sekali saat halaman dimuat.
  Model diunduh di HALAMAN LAIN, jadi peringatan "belum diunduh" bertahan dan
  tombol Mulai tetap mati sampai aplikasi ditutup. Sekarang ditarik ulang setiap
  jendela kembali fokus, dan pilihan pengguna dipertahankan selama modelnya ada.

Polanya sama dan patut dicurigai di tempat lain: **status yang bisa berubah dari
halaman lain tidak boleh diambil sekali lalu disimpan.**

## 3. "extract audio failed … Error opening output files: Invalid argument"

Bukan spasi di nama berkas, bukan ukuran, bukan bahasanya. **Videonya tidak punya
trek suara.** Direproduksi persis di Linux:

```
$ ffmpeg -i "Connect Lan.mp4" -vn -ac 1 -ar 16000 -c:a pcm_s16le out.wav
Output file does not contain any stream
Error opening output file out.wav.
Error opening output files: Invalid argument
```

ffmpeg menyebut berkas KELUARAN karena memang keluarannya yang tidak bisa
dibuat — tanpa satu pun stream untuk ditulis. Sebabnya ada di baris pertama,
yang tidak ikut terbawa ke pesan job.

Yang dikerjakan:

- `ffmpeg.HasAudio()` (ffprobe `-select_streams a`), dipanggil **sebelum**
  ekstraksi. Pesannya sekarang: *"this video has no sound track, so there is
  nothing to transcribe — Clipper picks moments from what is said"*.
- `/api/probe` ikut mengembalikan `has_audio`, dan pratinjau menampilkan lencana
  **⚠ no sound** di pojok bingkai begitu videonya dipilih — sebelum tombol Mulai
  ditekan sama sekali. Lencananya melayang: panel **616 px sebelum dan sesudah**
  lencana muncul.
- Baris waktu frame (`.frame-time`) ikut dibuat permanen (dimatikan saat
  pratinjau belum ada): kemunculannya dulu menggeser kolom 22 px tepat saat
  gambar pertama datang.

## Putaran kedua — v0.6.0 di Windows

### 4. "Sudah install ffmpeg tapi harus restart dulu"

`Server` menyimpan `Paths` sejak dibuat. Setelah `setup.Install` selesai,
tidak ada yang membacanya ulang — jadi `s.ff` masih menunjuk ffmpeg yang belum
ada, dan pratinjau tetap gagal sampai prosesnya dimulai ulang.

Sekarang `s.applyPaths()` dipanggil di dua tempat baru:

- **setelah pemasangan berhasil** (`installs.go`), dan
- **setiap kali `/api/requirements` ditanya** — jadi tombol "Check again" benar-benar
  memeriksa ulang segalanya, termasuk program yang dipasang di luar aplikasi
  (Ollama, atau ffmpeg lewat winget), bukan sekadar membaca ulang jawaban lama.

### 5. Tombol "Use another file" di baris Ollama

Ia membuka pemilih **video** ("Choose a video on this computer") untuk komponen
yang bukan berkas sama sekali — Ollama terpasang sebagai layanan jaringan, dan
engine memang menolak permintaannya (`that component cannot be pointed at a
file`). Sekarang engine yang menentukan lewat `Component.Pointable`
(ffmpeg/ffprobe/whisper/chrome = true, Ollama = false), dan pemilihnya memakai
judul yang benar: **"Point Clipper at ffmpeg.exe"**, bukan "Choose a video".

Ya, menunjuk berkas .exe langsung dipakai: pilihannya ditulis ke `.env` DAN
dipasang ke env proses, lalu `applyPaths()` — tanpa restart.

### 6. Model cloud Ollama tampil "ready" lalu gagal

Di mesin lain terdeteksi `gpt-oss:120b-cloud — 116.8B · MXFP4 · 0.0 GB · ready`,
dan jobnya gagal. Model `-cloud` terdaftar seperti model biasa tapi berukuran
**0 byte**: ia berjalan di server Ollama dan butuh akun. Sekarang ditandai TIDAK
siap, dengan alasannya: *"runs on Ollama's servers, not on this computer"*.

Jadi jawaban untuk "apakah pengecekannya cuma host?" — tidak. Yang diperiksa:
alamat (dan sistemnya), daftar model terpasang, jumlah parameter, panjang
konteks, kemampuan (`completion` vs `embedding`), dan sekarang lokal vs cloud.

### 7. Notifikasi "ada yang kurang" bisa langsung memasang

Alert di halaman Requirements kini membawa tombol **"Install N missing"** yang
memasang semua komponen wajib yang kurang DAN bisa dipasang engine. Yang tidak
bisa dipasang engine (Ollama, Chrome) sengaja tidak ikut — tombol yang
menjanjikan sesuatu yang tidak bisa dikerjakan lebih buruk daripada tidak ada
tombol.

## 8. Model kecil dilayani, bukan ditolak — potongan yang gagal DIPECAH

Laporan berikutnya: `qwen3.5:latest` membalas kosong pada 1 dari 2 potongan.
Percobaan ulang identik (3×) tidak menolong sama sekali — permintaan yang sama
persis pada model yang sama menghasilkan balasan kosong yang sama.

Yang menolong: **permintaan yang lebih kecil**. Sekarang:

1. Potongan yang balasannya tak terbaca **dipecah dua** lalu dikerjakan ulang,
   berulang sampai satu segmen. Daftar kerjanya antrian, bukan perulangan biasa,
   jadi ia boleh bertambah di tengah jalan. Model yang tidak sanggup membalas 12
   segmen sekaligus hampir selalu sanggup membalas 6.
2. **Balasan kosong tidak diulang identik**: kalau potongannya masih bisa
   dipecah, langsung dipecah — percobaan kedua hanya menghabiskan waktu.
3. Kalau SATU segmen pun gagal, barulah ia dilewati dan dihitung; lewat
   seperempat total, job berhenti dengan pesan bahwa modelnya memang terlalu
   kecil.
4. Ringkasan koreksi menyebutkan berapa potongan yang dipecah
   (`N part(s) split into smaller pieces for this model`).

**Konteks model dipakai apa adanya.** `Client.NumCtx` diisi dari metadata Ollama
(`context_length`) dan dikirim sebagai `num_ctx`. Meminta 8192 pada model yang
hanya mendukung 4096 membuat Ollama memuatnya dengan konteks yang tidak bisa
dipenuhi — balasan kosong, tanpa satu pun pesan galat. Itu kelas kegagalan yang
sama, dari arah yang berbeda.

Akibatnya `judge` ikut berubah: konteks di bawah 8192 **tidak lagi menolak**
model, hanya menandainya *"small context — Clipper will send smaller pieces,
which is slower"*. Yang ditolak sekarang hanya di bawah 2048, yang memang tidak
cukup untuk satu segmen pun.
