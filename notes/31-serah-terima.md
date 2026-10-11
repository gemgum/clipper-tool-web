# Serah terima — 6 Agustus 2026

Ditulis di akhir sesi panjang. **Baca ini lebih dulu**, lalu `CLAUDE.md`, baru
`notes/29` kalau perlu alasannya.

## Yang HARUS dikerjakan berikutnya

### 1. Galat ffmpeg/ffprobe di Windows — TERBLOKIR, butuh data

Dilaporkan pengguna kedua (Windows): *"untuk whisper tadi sebenarnya semua error
dari ffmpeg, ffprobe juga begitu."* **Belum diperbaiki, dan sengaja belum
ditebak.**

Yang sudah diperiksa dan TIDAK bersalah:

- `openableDir` (`engine/internal/api/files.go`) sudah menurunkan berkas ke
  foldernya dan menaiki induk bila tidak ada — diuji dengan path palsu, benar;
- di Linux `/api/requirements` melaporkan ffmpeg & ffprobe `installed: true`
  dengan path `/usr/bin`, dan pipeline jalan.

Yang sudah diperbaiki karena memang bisa dibuktikan: **`onDoubleClick` di
`gui/app/picker.tsx` memilih berkas APA PUN**, mengabaikan `pickable()` yang
dipatuhi klik biasa — itu yang membuat `whisper-cli.exe` bisa terpasang sebagai
"video sumber", dan pemilih berikutnya dibuka dari path yang mustahil.

**Yang dibutuhkan sebelum menyentuhnya lagi: teks galat ffmpeg-nya persis.**
Tanpa itu perbaikannya hanya tebakan yang kelihatan seperti pekerjaan.

Kalau nanti dikerjakan, mulai dari sini: buat `/api/requirements` benar-benar
MENJALANKAN `ffmpeg -version` dan `ffprobe -version`, bukan hanya memeriksa
berkasnya ada. "Ada" dan "bisa dijalankan" adalah dua hal berbeda di Windows
(arsitektur salah, DLL kurang), dan sekarang keduanya dilaporkan sama.

### 2. Riwayat job hilang tiap aplikasi ditutup

`engine/internal/job` menyimpan job di **memori** (`map[string]*Job`). Halaman
riwayat, tombol pilih, dan tombol hapus yang baru semuanya hanya bekerja untuk
sesi yang sedang berjalan.

Riwayat KARTU tidak begitu — `/api/cards` membaca folder apa adanya, jadi ia
bertahan. Ketidaksamaan ini akan membingungkan.

Belum dikerjakan **karena menyentuh format data**, dan itu keputusan pemilik
proyek: indeks job perlu ditulis ke disk (JSON per job di `<DataDir>/jobs/`,
atau satu berkas indeks). Jangan diputuskan sendiri.

### 3. 900×600 masih belum muat

Halaman klip melimpah ±480 px di jendela terkecil; halaman kartu ±600 px saat
artikel termuat. **Bukan soal kerapian** — di lebar 900 kolom setelan tinggal
~180 px sehingga kendalinya wajib menumpuk satu per baris.

Dua jalan, dan keduanya keputusan pemilik proyek: buang sebagian kendali, atau
naikkan `minWidth`/`minHeight` di `desktop/tauri.conf.json`. Sebagian sudah
ditolong `<Section>` yang bisa diciutkan.

## Yang sudah selesai dan TERUKUR di sesi ini

Semua angka di bawah dari `scripts/measure-ui.mjs` + Chrome headless, bukan
perkiraan.

- **Jendela tidak bergulir**: klip & kartu `0/0` di 1240×860 DAN 1600×1000.
- Pratinjau klip **sejajar** dengan kolom setelannya (479 = 479), kotak log
  **44 → 211 px** di jendela lebar.
- Kartu berita: pratinjau **380 → 423 px**, bisa dibuka **penuh layar**.
- `<select>` bawaan diganti komponen sendiri di **30 tempat** (nol tersisa).
- Berita diurutkan **terbaru dulu** (sebelumnya tidak pernah diurutkan sama
  sekali), semua feed dirangkak serentak, **gulir tak terbatas** 24 → 48 → 72.
- Riwayat: deretan **menyamping yang bisa diseret**, centang per item,
  pilih-semua, hapus — untuk klip DAN kartu.
- Tema gelap, kode akses, rail bawah, bilah atas dibuang.

## Aturan yang paling sering saya langgar sendiri di sesi ini

Ditulis supaya tidak terulang keempat kalinya:

1. **`flex: 1` pada "panel pertama" menunjuk berdasarkan URUTAN.** Tiap kali isi
   kolom berubah, ia menunjuk yang salah. Sudah menggigit tiga kali (kotak log,
   panel pratinjau, panel isi). Tunjuk panelnya dengan NAMA.
2. **Angka hasil pengukuran harus ditulis sebagai RUMUS**, bukan angka mati.
   `--pv-w: 210px` benar di 1240×860 dan salah di semua ukuran lain — pengguna
   melihat "tidak ada yang berubah", dan ia benar.
3. **Angka "tidak bergulir" tidak membuktikan tampilannya benar.** `ⓘ` yang
   jadi kotak kosong, kotak log terpangkas, dan 24 artikel tergencet jadi 24 px
   semuanya lolos dari angka dan hanya ketahuan dari POTRET.
4. **`overflow: hidden` pada item grid menghapus minimum otomatisnya**, jadi
   grid berhak menggencetnya di bawah tinggi isinya.
5. **Popup harus `position: fixed`**, dan harus bisa membuka ke ATAS. Tombol di
   dasar rail membuat popup yang selalu turun berakhir 282 px di bawah layar.
6. **Satu kalimat galat untuk dua sebab yang beda penanganannya lebih buruk
   daripada tidak ada kalimat.** "Ollama unreachable" dipakai untuk "mati" DAN
   "lambat", dan pengguna diarahkan memeriksa hal yang sudah benar.

## Cara memeriksa sebelum menyerahkan apa pun

```bash
./bin/clipper serve                                  # terminal lain
~/.cache/ms-playwright/chromium-*/chrome-linux64/chrome \
  --headless --remote-debugging-port=9333 --user-data-dir=/tmp/cdp about:blank &
node scripts/measure-ui.mjs http://127.0.0.1:8787 /tmp/shots
```

**Lihat potretnya, bukan cuma angkanya.** Lalu: `go vet` · `go test ./...` ·
`npx tsc --noEmit --noUnusedLocals --noUnusedParameters` · `npm run build`.
