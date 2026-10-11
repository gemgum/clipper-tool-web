# Pengerasan: yang sudah dikerjakan

Dikerjakan 5 Agustus 2026, menutup daftar kerja di
`28-pengerasan-sebelum-pentest.md` nomor 1–5. Berkas ini mencatat apa yang
berubah, **keputusan yang diambil di persimpangan**, dan cara membuktikannya
masih benar nanti.

Berkas 28 tetap berlaku sebagai duduk perkara dan sebagai daftar "sengaja
dibiarkan". Yang di sini adalah hasilnya.

## 1. Biner unduhan diperiksa sidik jarinya

`internal/setup/recipe.go`, `setup.go`

Dulu: engine mengunduh `ffmpeg.exe` dan `whisper-cli.exe` lalu
**menjalankannya**, dengan HTTPS sebagai satu-satunya penjaga.

Sekarang tiap alamat unduhan berpasangan dengan sha256 yang dipaku
(tipe `source`), dan berkasnya diperiksa **sebelum** namanya berubah dari
`.part` jadi nama berkas jadi. Gagal cocok = berkas dibuang, bukan diperbaiki
diam-diam. Sumber tanpa sidik jari **ditolak sebelum satu byte pun diunduh** —
penjaga itu ada di jalur pemakaian, bukan cuma di uji, supaya cermin baru yang
lupa dipaku gagal seketika.

Sidik jarinya diambil dengan mengunduh berkasnya sendiri lalu menghitungnya
(model: lewat `x-linked-etag` HuggingFace, yang memang sha256 objek LFS-nya).

### Cacat yang kita buat sendiri, dan cara menutupnya

Lanjutan-unduhan membiarkan `.part` dari cermin pertama **disambung** oleh cermin
kedua. Isinya beda build, ukurannya wajar, hasilnya rusak — dan tetap
dijalankan. Sekarang alamat asal ditulis ke `<berkas>.part.from`; pindah cermin
berarti potongannya dibuang dan mulai dari nol.

Sidik jari sebenarnya menangkap ini juga, tapi baru **setelah** seluruh sisanya
diunduh. Penanda asal menghentikannya di byte pertama. Keduanya dipakai.

### Keputusan: cermin yang tidak bisa dipaku dibuang

`https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip` **dicabut**.
Alamatnya selalu menunjuk "rilis terbaru" tanpa nomor versi, jadi isinya berganti
sendiri dan sidik jarinya mustahil dipaku. Cermin GitHub GyanD yang tetap dipakai
adalah build yang sama dari pengunggah yang sama, hanya bernomor.

`BtbN/FFmpeg-Builds` tetap ada tapi **pindah dari tag `latest` ke satu tag
autobuild** (`autobuild-2026-08-04-21-26`) — dengan alasan yang sama.

Kenapa cermin kedua dipertahankan meski keduanya sama-sama di GitHub: yang
ditakutkan bukan "GitHub mati" (kalau GitHub mati, aplikasinya sendiri tak bisa
diunduh), melainkan **satu pengunggah menghapus rilisnya**. Untuk itu pengunggah
yang berbeda memang menolong.

### Model: dipaku ke commit, bukan ke cabang

Alamat model berpindah dari `resolve/main/` ke
`resolve/5359861c739e955e79d9a303bcbc70fb988958b1/`. Cabang bisa dipindahkan
kapan saja oleh pemilik repo, dan berkas yang isinya boleh berubah tidak bisa
dipaku sidik jarinya. Dua cermin (HuggingFace + hf-mirror) berbagi satu sha256 —
cermin yang menyajikan isi berbeda dari aslinya bukan cermin, dan justru itu yang
ingin ketahuan.

**Menaikkan `modelRevision`, `whisperVersion`, `ffmpegVersion`, atau `btbnBuild`
berarti menghitung ulang sha256-nya.** Keduanya satu paket; menaikkan versi tanpa
sidik jari baru menghasilkan pemasangan yang gagal di mesin pengguna.

## 2. Permintaan yang mustahil datang dari GUI ditolak

`internal/api/guard.go` (baru) — tiga pemeriksaan, berlaku di **semua** mode,
jadi mode pengembangan ikut terlindung tanpa harus menyalakan kunci di sana:

| Pemeriksaan | Menutup apa | Jawaban |
| --- | --- | --- |
| `Host` harus alamat mesin ini | DNS rebinding — di serangan itu Host berisi nama si penyerang, berapa pun IP yang akhirnya dituju | 403 |
| Badan permintaan wajib `application/json` | POST lintas-asal yang tidak butuh preflight. Begitu jenisnya wajib JSON, browser wajib minta izin dulu — dan izin itu yang ditolak CORS | 415 |
| `Sec-Fetch-Site: cross-site` ditolak — seluruhnya saat terpasang, yang tanpa Origin saja saat checkout | `<img src>`, `<script src>`, navigasi dari halaman asing: bentuk yang tidak membawa Origin sama sekali, jadi tak terlihat CORS | 403 |

Urutan lapisannya sekarang `withGuard(withCORS(withToken(mux)))` — dari yang
paling murah & luas ke yang paling sempit.

Tiga hal yang sengaja **tidak** ditolak, dan alasannya:

- **`cross-site` yang membawa Origin — tapi hanya saat kunci mati.** Ini sempat
  salah dan menghasilkan bug (lihat di bawah): `localhost` dan `127.0.0.1`
  adalah host **berbeda** bagi browser, jadi GUI pengembangan di
  `localhost:3000` yang menghubungi engine di `127.0.0.1:8787` dilabeli
  `cross-site` — sama persis dengan halaman penyerang. Jendela Tauri dengan asal
  `http://tauri.localhost` jatuh di keranjang yang sama.

  Ketatnya karena itu **mengikuti keadaan kunci**, dan itu bukan kompromi
  melainkan pengamatan: saat terpasang, GUI disajikan engine sendiri, jadi
  setiap permintaan yang sah adalah same-origin dan tidak ada satu pun
  cross-site yang wajar — semuanya ditolak. Saat checkout, yang ditolak hanya
  yang tidak membawa Origin; sisanya diserahkan ke CORS, yang memang membedakan
  halaman lokal dari halaman internet.

  | Permintaan | Terpasang | Checkout |
  | --- | --- | --- |
  | same-origin (alur aplikasi jadi) | 200 | 200 |
  | cross-site + Origin lokal (`npm run dev`) | **403** | 200 |
  | cross-site tanpa Origin (`<img src>`) | 403 | 403 |
  | cross-site + Origin dari internet | 403 | 403 |

  Hasilnya: build yang dikirim ke pengguna seketat versi pertama, alur
  pengembangan tetap hidup, dan tidak ada mode yang kehilangan penjaga.
- **`OPTIONS`** tidak dinilai dari Content-Type — preflight memang tanpa badan,
  dan menolaknya berarti menolak permintaan yang justru sedang meminta izin.
- **`/api/upload`** tetap multipart: videonya bisa bergiga-giga dan harus
  dialirkan, bukan dimuat ke memori sebagai JSON.

`-addr` yang disebut pengguna sendiri ikut diterima (`Server.AllowHost`, dipanggil
dari `main.go`). Mengikat ke alamat non-loopback adalah keputusan sadar, dan
menolaknya di sini berarti flag itu diam-diam tidak berfungsi. `0.0.0.0`/`::`
tidak ikut — itu cara mendengar, bukan nama yang bisa dituju.

### Bug yang dibuat lalu diperbaiki: `npm run dev` mati total

Versi pertama penjaga ini menolak **setiap** `Sec-Fetch-Site: cross-site`.
Akibatnya `cd gui && npm run dev` lalu membuka `http://localhost:3000` menjawab
"Cannot reach the engine at http://127.0.0.1:8787" di ketiga halaman — 403 di
tiap permintaan API.

Sebabnya satu kalimat yang saya kira benar dan ternyata tidak: **bagi browser,
`localhost` dan `127.0.0.1` bukan host yang sama.** Jadi label yang dikirimnya
bukan `same-site` melainkan `cross-site`.

Yang membuatnya sulit terlihat: uji unitnya lulus (saya menguji `same-site`,
label yang sebenarnya tidak pernah muncul di alur itu), dan uji browser saya
membuka `127.0.0.1:8787` langsung — satu-satunya alur yang memang tidak
terpengaruh. Pelajaran yang lebih umum daripada bugnya: **kalau sebuah penjaga
menilai "asal", ujilah lewat asal yang sebenarnya dipakai, bukan lewat nilai
header yang diketik sendiri.**

### Yang ikut berubah di GUI

`gui/app/page.tsx`: pembatalan job dulu `fetch(..., { method: "POST" })` tanpa
badan dan tanpa jenis. Sekarang menyebut `application/json`. Semua POST lain
sudah menyebutnya sejak awal.

## 3. Kunci sesi pindah ke cookie

`internal/api/token.go`, `gui/app/engine.ts`

Query adalah tempat paling mudah bocor: Referer, riwayat browser, tangkapan
layar, salin-tempel. Sejak GUI disajikan engine sendiri, `EventSource`,
`<video src>`, dan tautan unduh semuanya **satu asal** — dan cookie ikut terkirim
sendiri oleh ketiganya. Itulah yang dulu memaksa kuncinya ditaruh di query.

Sekarang: `clipper_session`, `HttpOnly; SameSite=Strict; Path=/`, tanpa masa
berlaku (cookie sesi — hilang saat jendela ditutup, sama seperti kuncinya
sendiri yang dibuat baru tiap engine dijalankan).

**`/api/` tidak lagi menerima `?token=` sama sekali**, bahkan ketika kuncinya
benar. Query hanya dibaca sekali, di halaman non-`/api/` yang dibuka shell, untuk
ditukar jadi cookie. Kunci salah di alamat halaman tidak menghasilkan cookie apa
pun — kalau tidak, siapa pun bisa memasang cookie pilihannya sendiri.

`eng()` di `gui/app/engine.ts` **berhenti menempelkan kunci** ke setiap alamat.
Ia tetap wajib dipakai untuk semua pemanggilan engine, tapi alasannya sekarang
tinggal satu: alamat engine baru diketahui saat halaman dibuka, sebab portnya
acak begitu aplikasi terpasang. `sessionStorage` untuk kunci ikut dibuang —
cookie yang menyimpannya, dan JavaScript memang tidak boleh bisa membacanya.

**Akibat yang harus diketahui:** menjalankan `clipper serve -token on` dari
checkout lalu memakai GUI `npm run dev` di `:3000` tidak lagi berfungsi penuh —
cookie `SameSite=Strict` tidak terkirim lintas-asal. Itu tidak menghalangi
apa pun: dari checkout kuncinya memang mati secara bawaan, dan alur yang dipakai
aplikasi jadi adalah satu asal. Bila suatu saat perlu, jalurnya lewat header
`X-Clipper-Token` yang tetap diterima.

## 4. Path berawalan tanda hubung

`internal/ffmpeg/ffmpeg.go` — `CLIPath()`.

Tidak ada shell di seluruh engine (`os/exec` tanpa `sh -c`), jadi injeksi shell
memang tidak berlaku. Yang berlaku: ffmpeg membaca argumen berawalan `-` sebagai
flagnya sendiri, dan path itu datang dari klien (`/api/probe?path=…`,
`/api/frame?path=…`, yang keduanya tidak mem-`Stat` berkasnya lebih dulu).

ffmpeg tidak mengenal `--` sebagai penanda akhir flag, jadi yang dipakai adalah
bentuk yang selalu dipahami: **path mutlak**. `-report` jadi
`/folder/kerja/-report` — tidak lagi diawali tanda hubung, menunjuk berkas yang
sama persis. Path yang tidak diawali `-` tidak disentuh sama sekali, jadi tidak
ada perilaku lama yang berubah.

Semua argumen path ke ffmpeg/ffprobe lewat sana. whisper **tidak** ikut: path
yang diterimanya (WAV cache, prefix keluaran, model) semuanya disusun engine,
bukan datang dari klien — dan mengimpor paket `ffmpeg` dari `transcribe` hanya
untuk itu justru menambah keterikatan yang tidak perlu.

## 5. Pengetatan lapis kedua

- **CSP** untuk halaman yang disajikan engine (`internal/api/webui.go`), plus
  `X-Content-Type-Options: nosniff` dan `Referrer-Policy: no-referrer`.
- **Semua badan JSON lewat `readJSON`** (batas 1 MB). Enam handler tadinya
  memakai `json.NewDecoder(r.Body)` langsung — tanpa batas ukuran sama sekali.

### CSP: satu kelonggaran, dan kenapa

`script-src` terpaksa memuat `'unsafe-inline'`. Ini **diperiksa, bukan dikira**:
`next build` menaruh tujuh `<script>` inline di `index.html` untuk data
hidrasinya. Nonce tidak bisa dipakai (berkasnya statis, engine tidak menyusun
HTML-nya) dan hash berganti tiap build. Yang tetap didapat: skrip dari **host
luar** ditolak, dan `'unsafe-eval'` tetap tidak ada — keduanya sudah diperiksa
tidak dibutuhkan pada `gui/out` hari ini.

`img-src` mengizinkan http/https: gambar artikel di tab kartu memang datang dari
medianya. Itu **isi**, bukan tampilan — pengecualian yang disebut `notes/29`
secara khusus.

Nilai sampingannya untuk `notes/29`: aturan "nol alamat luar di antarmuka"
berhenti bergantung pada ingatan. Satu tautan CDN yang tak sengaja ikut akan
langsung gagal memuat, bukan diam-diam bekerja di mesin yang punya internet lalu
putus di mesin yang tidak.

## Cara membuktikannya masih benar

```bash
cd engine && go test ./...          # semua paket lulus
./build.sh && ./bin/clipper serve -token on
```

Lalu, dengan `T` = token dari `data/engine.json` dan `B=http://127.0.0.1:8787`:

| Permintaan | Harus |
| --- | --- |
| `curl -D- "$B/?token=$T"` | `Set-Cookie: clipper_session=…; HttpOnly; SameSite=Strict` |
| `curl -b "clipper_session=$T" "$B/api/config"` | 200 |
| `curl "$B/api/config?token=$T"` | **401** — query tidak diterima lagi |
| `curl "$B/api/config"` | 401 |
| `curl -H 'Host: penyerang.example' -b "clipper_session=$T" "$B/api/config"` | 403 |
| `curl -X POST -H 'Content-Type: text/plain' -b "clipper_session=$T" -d '{}' "$B/api/jobs"` | 415 |
| `curl -X POST -H 'Content-Type: application/json' -b "clipper_session=$T" -d '{}' "$B/api/jobs"` | 400 — lolos penjaga, ditolak validasi |
| `curl -H 'Sec-Fetch-Site: cross-site' -b "clipper_session=$T" "$B/api/config"` | 403 |
| `curl -H 'Origin: https://asing.example' -b "clipper_session=$T" "$B/api/config"` | 403 |
| `curl -D- "$B/"` | memuat `Content-Security-Policy` |
| `curl "$B/api/health"` | 200 — tetap terbuka untuk shell |

Kesebelasnya sudah dijalankan pada 5 Agustus 2026 dan hasilnya sesuai.

## Uji browser: cara menjalankannya di WSL

Chrome **ada** di mesin pengembangan ini — Chrome Windows lewat `/mnt/c`, yang
memang sudah dipakai engine untuk kartu berita (`capture.Find()`). Chrome Windows
bisa menghubungi engine di WSL lewat `127.0.0.1` (localhost forwarding WSL2),
jadi uji browser tidak butuh alat tambahan apa pun:

```bash
CHROME="/mnt/c/Program Files/Google/Chrome/Application/chrome.exe"
"$CHROME" --headless=new --disable-gpu --user-data-dir='C:\Users\Public\clipper-qa' \
  --enable-logging=stderr --v=1 --window-size=1240,860 --virtual-time-budget=9000 \
  --screenshot='C:\Users\Public\shot.png' "<alamat open: dari banner>"
```

Pelanggaran CSP muncul di stderr sebagai baris `Refused to …`. Dua jebakan yang
sudah kena sekali, supaya tidak berulang:

- **`--dump-dom` tidak akan pernah selesai di halaman Requirements**: halaman itu
  membuka `EventSource` yang memang tidak pernah menutup, jadi Chrome menunggu
  selamanya. Pakai `--screenshot`, atau uji SSE-nya lewat `curl -N` saja.
- **Cookie sesi mati bersama proses Chrome.** Tiap pemanggilan headless adalah
  proses baru, jadi setiap kali harus lewat `?token=` lagi. Ini bukan cacat
  produk — jendela aplikasi sungguhan tetap terbuka — tapi kalau lupa,
  halamannya melapor "Cannot reach the engine" dan terlihat seperti bug.
- Chrome yang tertinggal harus dimatikan **berdasarkan baris perintahnya**
  (`CommandLine -like '*clipper-qa*'`), bukan `taskkill /IM chrome.exe` — yang
  terakhir ikut menutup browser pengguna beserta seluruh tabnya.

### Sudah lulus (5 Agustus 2026)

| Yang diuji | Hasil |
| --- | --- |
| `/` dibuka dengan `?token=` | terender penuh, data dari API terisi (daftar model dengan ✓/✗) |
| `/news` | terender penuh |
| Pelanggaran CSP di ketiga halaman | **nol** |
| `token=` tersisa di DOM/bilah alamat | **nol** |
| SSE `/api/requirements/events` dengan cookie | aliran bertahan |
| SSE yang sama tanpa cookie | 401 seketika |
| `gui/out` memuat `eval`/`new Function` | tidak ada |
| `gui/out` memuat alamat luar | hanya di teks pesan galat Next/React |

### Masih harus, sebelum rilis

1. **Satu job sampai selesai.** Itu menyentuh SSE progres, `<video src>`, dan
   tautan unduh sekaligus — tepat ketiga hal yang dulu memaksa kunci ditaruh di
   query. Belum diuji karena butuh video sungguhan.
2. **Buat satu kartu berita** dengan gambar dari medianya — itu yang menguji
   `img-src` terhadap host luar.
3. **Cabut jaringan, buka aplikasi** — uji "nol alamat luar" dari `notes/29`.
4. **Di Windows, di jendela Tauri yang sebenarnya.** Yang diuji di sini Chrome,
   sedangkan aplikasinya memakai WebView2. Keduanya Chromium, tapi bukan hal yang
   sama — dan Windows adalah sasaran uji penetrasinya.

## Asal non-http: disebut satu per satu

`localOrigin` dulu berbunyi "selain http/https, terima saja". Niatnya tiga skema
shell desktop (`tauri://`, `app://`, `file://`), tapi yang tertulis adalah
SEMUA — `chrome-extension://` dan skema karangan mana pun ikut lolos CORS.

Kuncinya tetap menahan permintaannya, jadi itu bukan lubang di aplikasi
terpasang; ia longgar tanpa satu pun alasan, dan longgar tanpa alasan adalah
yang paling mudah jadi temuan. Sekarang daftarnya tertutup (`shellSchemes`).

## Batas yang tidak ditutup penjaga mana pun — siapkan jawabannya

Ini bukan cacat, ini bentuk sistemnya. Ditulis supaya jawabannya tidak dikarang
saat laporan datang:

- **Header adalah penjaga untuk BROWSER, bukan untuk program.** Program apa pun
  yang jalan sebagai penggunanya bisa menyetel `Host`, `Origin`, dan
  `Sec-Fetch-Site` sesukanya — seluruh tabel di atas dihasilkan `curl` yang
  melakukan persis itu. Yang menghentikan program lokal hanyalah kunci sesi, dan
  kunci itu ada di `engine.json` yang bisa dibaca proses milik pengguna yang
  sama (0600 melindungi dari pengguna LAIN). Itu memang batas kepercayaan yang
  ditulis di `28`: apa pun di balik kunci dianggap penggunanya sendiri.
- **`SameSite=Strict` ditegakkan browser, bukan engine.** Penguji mungkin
  melaporkan "tidak ada token CSRF". Jawabannya: pertahanannya berlapis —
  cookie tidak pernah dikirim lintas-situs, Origin diperiksa di server,
  `cross-site` ditolak, dan badan permintaan wajib `application/json` sehingga
  bentuk POST sederhana tidak bisa dipakai sama sekali.
- **Mode checkout tetap tanpa kunci.** `./bin/clipper serve` dari sumber berarti
  port 8787 tanpa kunci, dan program lokal mana pun bisa memerintah engine. Itu
  bukan build yang dikirim — pastikan yang diuji adalah hasil pemasangan, bukan
  checkout.

## Yang tetap dibiarkan

Tabel "sengaja dibiarkan" di `28-pengerasan-sebelum-pentest.md` tidak berubah:
`/api/browse` menelusuri disk, `/api/requirements/path` menunjuk program apa pun,
engine menghubungi alamat dari klien, pemasang tidak ditandatangani. Semuanya
punya alasan yang ditulis di sana, dan tiga yang pertama ada **di balik kunci
sesi** — yang sejak sekarang tidak lagi lewat bilah alamat.
