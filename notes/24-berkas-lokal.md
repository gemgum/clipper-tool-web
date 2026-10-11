# Berkas lokal: kirim path, jangan unggah salinan

Selesai 4 Agustus 2026. Penghalang nomor 3 di `23-aplikasi-desktop.md`.

## Soalnya

Menyeret video ke GUI berarti `POST /api/upload`: berkasnya disalin ke
`data/uploads/` lebih dulu. Untuk video 3,84 GB itu berarti menunggu penyalinan
selesai sebelum pekerjaan dimulai, dan 3,84 GB ruang disk terpakai dua kali —
padahal engine jalan di komputer yang sama dengan videonya.

Penyebabnya bukan kemalasan: **browser tidak pernah memberi tahu halaman web di
mana sebuah berkas berada.** Yang diberikan `<input type=file>` dan drag & drop
hanyalah nama, ukuran, waktu ubah, dan isinya. Path-nya sengaja disembunyikan.

## Jalan keluarnya: engine yang membaca folder

Engine tidak terikat aturan itu. Dua endpoint baru (`internal/api/files.go`):

| Endpoint | Isi |
| --- | --- |
| `GET /api/browse?dir=` | isi satu folder + pintasan (Home, Videos, Downloads, dan folder rumah Windows saat di WSL) |
| `POST /api/locate` | `{name, size}` → path aslinya, bila ketemu |

`/api/browse` menopang pemilih berkas milik GUI sendiri (`gui/app/picker.tsx`),
dipakai untuk video maupun folder keluaran. Tidak ada byte yang disalin.

`/api/locate` menyelamatkan kebiasaan yang sudah ada: pengguna tetap boleh
menyeret berkas ke jendela. Nama + ukuran dikirim ke engine, engine mencarinya
di folder yang wajar, dan kalau ketemu path itu yang dipakai. Unggahan tinggal
jadi cadangan — dan sekarang benar-benar hanya cadangan.

## Aturan yang membuatnya aman

- **Nama DAN ukuran harus sama persis.** Nama saja tidak cukup: memproses video
  yang salah tanpa ada yang sadar lebih buruk daripada gagal menemukan.
- **Dua kandidat = tidak ketemu.** Kalau ada dua berkas yang cocok, engine
  menjawab 404 dan GUI kembali ke unggahan. Menebak yang mana lebih buruk
  daripada bertanya.
- **Anggaran waktu 2,5 detik, kedalaman 4 folder**, dan folder ribut
  (`node_modules`, `AppData`, `Windows`, …) dilewati. Pencarian ini berjalan
  sementara pengguna menunggu; kalau tidak selesai cepat, ia tidak berguna.
- Pengukuran nyata di WSL: **0,9 detik** untuk menemukan berkas di
  `/mnt/c/Users/<nama>/Videos`.

## Yang ikut berubah

`POST /api/jobs` sekarang memeriksa path sebelum job dibuat: berkas tidak ada
atau ternyata folder dijawab 400 dengan pesan yang menyebut path-nya. Sejak GUI
mengirim path alih-alih menggandakan berkas, salah ketik satu huruf jadi
kekeliruan yang paling mungkin terjadi — dan tanpa pemeriksaan ini pesannya baru
muncul dari ffmpeg belasan detik kemudian.

## Akibatnya untuk keamanan

Menjelajah folder lewat HTTP berarti siapa pun yang bisa menghubungi engine bisa
membaca daftar isi disk pengguna. Itulah yang membuat kunci sesi berhenti jadi
"nanti saja" — lihat `26-kunci-sesi.md`.
