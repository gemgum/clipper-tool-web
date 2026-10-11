# Kenapa Path Diketik Manual + Solusi Tauri

## Masalah: browser menyembunyikan path asli

Browser (Chrome/Firefox) **sengaja menyembunyikan lokasi asli file** demi
privasi/keamanan. Saat pilih file lewat `<input type="file">`, JavaScript
hanya dapat **nama + isi file**, bukan path lengkap:

```
Kamu pilih : /home/php-02/video.mp4
JS terima  : "video.mp4"   ← path asli disembunyikan
```

## Kenapa jadi masalah

Engine Go membaca file **langsung dari disk lewat path**. GUI harus memberi
path asli. Karena browser menyembunyikannya, ada 2 siasat:

| Kebutuhan | Siasat di browser |
|---|---|
| Video (masukan) | (a) ketik path manual, atau (b) unggah — engine simpan & dapat path (menyalin file) |
| Folder output (tujuan) | hanya ketik manual — folder itu tujuan, tak ada "isi" untuk diunggah |

Catatan: `showDirectoryPicker()` (Chrome) memberi "handle" izin sementara,
BUKAN path — Go tetap tak bisa memakainya. Jadi tetap buntu.

## Solusi: Tauri (bila dibungkus desktop nanti)

Tauri membungkus frontend Next.js jadi **aplikasi desktop**. Karena app desktop
punya **izin akses OS penuh** (via backend Rust), ia bisa memanggil **dialog
"Pilih File/Folder" bawaan OS** yang mengembalikan **path absolut asli**.

```
Web biasa      : [ketik path manual]              ← browser buta lokasi
Tauri (desktop): [tombol "Pilih Folder"] → dialog OS → "/home/php-02/hasil" ✅
```

Analogi: browser = tamu (dibatasi ruang tamu); Tauri = penghuni (semua ruangan).

## Kesimpulan
- Sekarang (web lokal): path video & folder output diketik manual (atau video
  diunggah). Aman & wajar.
- Nanti (Tauri): bisa tombol "Pilih video"/"Pilih folder" native → isi path
  otomatis, tanpa mengetik, tanpa menyalin file. (Electron juga bisa, lebih berat.)
- Status: peningkatan OPSIONAL masa depan, bukan keharusan sekarang.
