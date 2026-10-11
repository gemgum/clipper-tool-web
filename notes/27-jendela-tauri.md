# Jendela aplikasi: Tauri

Diputuskan 4 Agustus 2026, menutup "yang belum diputuskan" di
`23-aplikasi-desktop.md`. Chrome `--app=` tidak jadi dipakai: yang dicari adalah
pemasang `.msi` dan jendela asli sistem, bukan browser yang menumpang.

## Bentuknya

```
Clipper.exe (Tauri)
  └── menjalankan  clipper serve -shell        (engine Go)
        └── engine memilih port acak + kunci sesi
        └── engine mencetak: clipper-url: http://127.0.0.1:38887/?token=…
  └── jendela diarahkan ke alamat itu
```

Engine yang menyajikan **semuanya** — antarmuka maupun API — di satu alamat.
Jendelanya tipis dengan sengaja: 150 baris Rust yang tidak tahu apa-apa tentang
video, klip, atau model.

Tiga akibat yang membuat bentuk ini dipilih:

- **Tidak ada urusan CORS di aplikasi jadi.** GUI dan API satu asal yang sama.
  Aturan asal di `token.go` tinggal jadi pengaman untuk pengembangan.
- **Kalau Tauri ditinggalkan nanti, yang dibuang cuma 150 baris itu.** Chrome
  `--app=`, jendela GTK, apa pun — semuanya cuma perlu membuka satu alamat.
- **Bisa dibuka di browser biasa saat mengembangkan.** Alamat yang sama,
  ditempel sendiri. Tidak ada jalur khusus yang hanya hidup di dalam aplikasi.

## Antarmuka jadi berkas statis

`gui/next.config.mjs` sekarang `output: "export"`, jadi `npm run build`
menghasilkan HTML+JS biasa di `gui/out`. Engine menyajikannya dari sana
(`internal/api/webui.go`); di aplikasi terpasang, dari `gui/` di sebelah biner.

Menuntut Node.js dan `npm run dev` di komputer pengguna jelas tidak masuk akal —
dan seluruh GUI ini memang berjalan di browser, tidak ada satu pun bagian yang
butuh server Next. `./build.sh` sekarang membangun keduanya sekaligus.

Berkas GUI tidak dijaga kunci sesi, hanya `/api/`. Halamannya harus bisa dimuat
LEBIH DULU; kuncinya baru dibaca JavaScript di dalamnya, dari `?token=` pada
alamat yang dibuka jendela.

## Kenapa alamatnya lewat stdout, bukan engine.json

Shell adalah **induk proses** engine, jadi pipa stdout pasti miliknya sendiri.
`engine.json` bisa saja milik engine lain yang kebetulan sedang jalan — mis.
pengembang yang sedang menjalankan `clipper serve` di terminal.

Kontraknya satu baris, dicetak sebelum banner:

```
clipper-url: http://127.0.0.1:38887/?token=…
```

`api.ShellURLPrefix` di engine, `URL_PREFIX` di `desktop/src-tauri/src/main.rs`.
Kalau salah satu diubah, ubah keduanya.

`engine.json` tetap ada dan tetap berguna — untuk perkakas lain, dan untuk
memeriksa keadaan dengan tangan.

## Isi bundel

Tiga hal disalin ke folder resource:

| Sumber | Tujuan |
| --- | --- |
| `bin/clipper` (`clipper.exe` di Windows) | `engine/` |
| `gui/out/` | `gui/` |
| `assets/fonts/` | `assets/fonts/` |

Daftarnya ada di **`tauri.<os>.conf.json`, bukan di `tauri.conf.json`** — dan itu
bukan pilihan gaya. Tauri MENGGABUNGKAN config per-OS dengan config utama, tidak
menimpanya: selama daftar resource ada di config utama, entri `bin/clipper`
milik Linux ikut terbawa saat build Windows dan build gagal dengan
"resource path `..\..\bin\clipper` doesn't exist". Jadi config utama tidak
memuat daftar resource sama sekali; tiap sistem menyebut binernya sendiri.

Susunan itu bukan kebetulan: engine mencari GUI dan font **di sebelah binernya**
(`<exe>/../gui`, `<exe>/../assets/fonts`), dan itu persis yang dihasilkan
susunan di atas. Tidak ada kode yang perlu tahu ia sedang di dalam bundel Tauri.

Model whisper dan biner ffmpeg TIDAK ikut dibundel — itu tugas halaman
Requirements, dan tempatnya di folder data pengguna (`25-pemasangan-komponen.md`).

## Membangunnya

Sekali saja, di mesin pengembang:

```bash
# Linux/WSL — pustaka sistem untuk WebKitGTK
sudo apt install libwebkit2gtk-4.1-dev build-essential curl wget file \
     libxdo-dev libssl-dev libayatana-appindicator3-dev librsvg2-dev
# Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

Lalu:

```bash
./build.sh                    # engine + gui/out
cd desktop && npm install
npm run dev                   # jendela, engine dari ../../bin/clipper
npm run build                 # pemasang di src-tauri/target/release/bundle
```

**`.exe` Windows harus dibangun di Windows** — begitu juga `.dmg` di macOS.
WebView2 dan penandatanganan hanya ada di sana; ini bukan keterbatasan Tauri,
tapi memang begitu cara kerja ketiga sistem itu.

## Yang belum diuji

Bagian Rust belum pernah dikompilasi: mesin tempat ini ditulis tidak punya Rust
maupun pustaka WebKitGTK, dan keduanya butuh `sudo`. Yang sudah diuji sungguhan
adalah bagian di bawahnya — engine menyajikan GUI di satu alamat, dengan kunci
sesi, dan `clipper serve -shell` mencetak baris alamat yang benar.
