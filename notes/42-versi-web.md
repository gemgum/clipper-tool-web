# 42 — Versi web (HANYA branch `webview`)

Diputuskan 8 Oktober 2026: di branch `webview` saja, Clipper dijalankan sebagai
web untuk **tim kecil dengan satu kata sandi bersama**, di **VPS tanpa Docker**
(biner + systemd + Caddy). `main` tetap desktop; aturan "Sasaran: DESKTOP" di
CLAUDE.md berlaku di sana tanpa perubahan.

## Apa yang berubah: `clipper serve -web -host <nama>`

Satu build untuk dua mode; engine yang menentukan, GUI membaca `web` dari
`/api/health`.

| Hal              | Desktop                         | `-web`                                              |
| ---------------- | ------------------------------- | --------------------------------------------------- |
| Masuk            | kunci acak dari shell → cookie  | `/login` + `CLIPPER_PASSWORD` → cookie 30 hari      |
| Kunci sesi       | acak tiap jalan                 | HMAC dari kata sandi (restart tidak mengeluarkan tim; ganti sandi = semua keluar) |
| Path dari klien  | path mana pun di mesin pengguna | HANYA di `<DataDir>/uploads` (`insideWebRoot`)      |
| Berkas sumber    | dibaca di tempat (notes/24)     | diunggah; satu folder per unggahan                  |
| Hasil            | "Buka folder"                   | tautan unduh (klip, `.srt`, `.txt`, watermark)      |
| Folder keluaran  | pilihan pengguna                | selalu folder bawaan di server                      |
| Dimatikan (403)  | —                               | browse, locate, reveal, requirements install/remove/path, settings/folders |
| Video unggahan   | —                               | DIHAPUS begitu job sukses (klip, caption, watermark); gagal/batal = tetap, bisa diulang |

Semua di `engine/internal/api/web.go`; diuji `TestWebMode`.

## Per halaman

| Halaman      | Desktop                                   | Web                                                         |
| ------------ | ----------------------------------------- | ----------------------------------------------------------- |
| Video clips  | path + pilih folder keluaran, Buka folder | tombol buka = unggah; folder keluaran hilang; kartu klip: Unduh / Tanpa subtitle / .srt / .txt; path dikosongkan setelah selesai |
| Captions     | pilih video/folder, tempel path, folder   | satu tombol "Unggah video…" (banyak berkas); unduh `.txt` per video |
| Watermark    | idem                                      | idem; unduh video hasil per berkas                          |
| Article writer | "Disimpan di <folder>"                  | Unduh `.md`                                                 |
| News cards, History | sudah unduhan                      | tidak berubah                                               |
| Requirements | pasang, tunjuk program, ganti folder      | hanya status; tombolnya disembunyikan                       |

Video yang dipakai DUA job sekaligus hilang begitu job pertama selesai — job
kedua gagal "not found". Unggah per job.

## Memasang di VPS

```bash
# prasyarat: git cmake g++ curl, Go, Node/npm, ffmpeg, chromium (kartu berita), caddy
sudo useradd -r -m -d /opt/clipper clipper
sudo -u clipper git clone -b webview <repo> /opt/clipper
cd /opt/clipper
sudo -u clipper ./setup.sh small        # whisper.cpp + model
sudo -u clipper ./fetch-fonts.sh
sudo -u clipper ./build.sh              # bin/clipper + gui/out
echo 'CLIPPER_PASSWORD=<sandi panjang>' | sudo -u clipper tee -a .env
sudo chmod 600 .env
sudo cp deploy/clipper.service /etc/systemd/system/   # sunting -host
sudo cp deploy/Caddyfile /etc/caddy/Caddyfile         # sunting nama host
sudo systemctl enable --now clipper && sudo systemctl reload caddy
```

Kunci API (Claude/OpenAI/…) diisi sekali lewat halaman Requirements, sama
seperti desktop; tersimpan di `.env` server dan dipakai seluruh tim.

## Terpasang: klip.sarthlutions.id (8 Oktober 2026)

VPS 203.194.113.228 (Ubuntu 22.04, 2 vCPU, dipakai bersama situs lain di nginx).
Bentuknya BUKAN checkout: semua di `/var/www/clipper`, folder ditunjuk lewat env
di unit systemd (`deploy/clipper-vps.service`), situs nginx di
`deploy/nginx-clipper.conf`.

- engine: biner statis dari lokal (`CGO_ENABLED=0`), sebab glibc lokal 2.39 > server 2.35;
- whisper.cpp v1.9.1: dikompilasi DI server (`/var/www/clipper/src`);
- ffmpeg 4.4 dari apt (`movie=` & `subtitles=` tersedia);
- Chrome for Testing di `tools/` — BUKAN snap: snap menolak user layanan yang
  rumahnya di luar `/home`;
- tanpa batas CPU (pilihan pemilik) — transkripsi bisa melambatkan situs lain.

Memperbarui: build biner statis + `gui/out`, `rsync` ke `bin/` dan `gui/out/`,
`systemctl restart clipper`.

## Batas yang diketahui (sengaja belum dikerjakan)

- **Unggahan dari job yang gagal/dibatalkan, dan gambar watermark,** tidak
  dihapus otomatis. Bersihkan dengan cron
  (`find data/uploads -mindepth 1 -maxdepth 1 -mtime +7 -exec rm -rf {} +`).
- **Hasil (klip, kartu, caption, video watermark) tidak pernah dihapus** — tetap
  bisa diunduh dari History. Hapus lewat History atau cron yang sama untuk
  `data/` bila disk menipis.
- **Semua orang melihat semua job.** Itulah arti "akun bersama".
- **Satu job jalan sekaligus** (`-jobs 1`); job lain antre. Naikkan bila CPU VPS cukup.
- **Tebakan sandi**: satu per detik untuk seluruh server. Cukup untuk tim kecil;
  pasang fail2ban/rate-limit bila alamatnya diserang.
- **Komponen** dipasang admin lewat `setup.sh`; halaman Requirements di web
  hanya menampilkan status.
- **SSRF**: kartu berita & pembuat berita mengambil URL apa pun dari server.
  Dapat diterima untuk tim yang memegang sandi; jangan bagikan sandinya keluar.
