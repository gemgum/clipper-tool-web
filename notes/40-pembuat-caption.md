# Pembuat Caption — Keputusan Desain

Tanggal: 19 Agustus 2026. Permintaan pemilik proyek: caption yang **memancing**,
otomatis dibatasi **5 menit** video tapi bisa diatur, **satu video atau bulk**,
hasilnya **berkas .txt bernama seperti videonya**.

Empat pertanyaan yang muncul saat merancangnya dijawab di sini supaya sesi
berikutnya tidak mengulang diskusinya.

## Ini permukaan KEEMPAT, dan aturan teksnya paling longgar

| | Kartu berita | Pembuat berita | Pembuat caption |
| --- | --- | --- | --- |
| Aturan teks | verbatim | LLM menulis, pagar fakta | LLM mengarang |
| Sumber | artikel | 5 artikel | ucapan video |
| Keluaran | PNG 1080×1920 | judul + badan | `<video>.txt` |

Yang dijaga di sini bukan kata-katanya melainkan ISINYA. Dua pagar, dan hanya
dua, karena hanya keduanya yang bisa dibuktikan mesin:

1. **Angka yang tidak pernah diucapkan** dilaporkan. "3 juta penonton" yang
   tidak ada di videonya adalah klaim palsu, titik — dan angka satu-satunya
   unsur yang bisa dicocokkan tanpa menebak.
2. **Transkripnya ikut ditulis** di bawah caption, di berkas yang SAMA. Klaim
   mengada-ada tanpa angka tetap urusan mata manusia, dan mata itu tidak akan
   membuka berkas lain untuk memeriksanya.

Pagar ketiga bukan kode: **tiga caption sekaligus, manusia yang memilih satu.**
Itu pagar termurah yang ada untuk teks karangan, dan satu caption berarti
menerima apa pun yang keluar.

## Empat keputusan

**1. Berkasnya `<video>.txt` DI SEBELAH videonya.** Sempat ditulis ke
`<data>/captions` demi menghindari tabrakan nama (lihat di bawah), dan itu
keliru: folder aplikasi adalah tempat terakhir yang akan dibuka orang saat
hendak memposting. Bawaannya sekarang folder video itu sendiri; kolom "Output
folder" di panel Videos mengumpulkan semuanya ke satu tempat bila memang
diinginkan.

Tabrakannya nyata dan tetap dijaga: tiap klip hasil Clipper sudah punya
`<klip>.txt` berisi ucapannya — bahan caption yang justru dipakai orang. Karena
itu berkas yang sudah ada DIPERIKSA lebih dulu. Keluaran kita sendiri (ditandai
baris `--- caption 1 ---`) ditimpa, sebab menjalankan ulang job yang sama harus
memperbarui berkasnya, bukan menumpuk salinan bernomor. Berkas milik siapa pun
selain kita tidak pernah ditimpa — dipakai `<klip>.caption.txt`.

**2. Video lebih panjang dari batasnya DIPOTONG, bukan ditolak.** Menolak
berarti satu berkas salah pilih menghentikan antrian 30 berkas di tengah jalan.
Yang dibaca disebut di baris kedua berkas hasil (`from the first 5:00 of 47:12`)
— selalu, juga saat seluruh video terbaca, sebab pembacanya tidak bisa menebak
sendiri.

Teknisnya `-t` pada ekstraksi audio yang sudah ada. **Batasnya ikut masuk kunci
cache transkrip** (`transcriptCacheKey`): transkrip separuh TIDAK boleh dipakai
ulang job klip yang membutuhkan video utuh. Nol tidak mengubah kuncinya sama
sekali, jadi cache lama tetap terpakai.

**3. Batasnya diatur per job** (`<Stepper>` di kolom kanan, bawaan 5 menit),
bukan sekali di Requirements. Ia menyangkut isi video yang sedang dikerjakan,
bukan pemasangan.

**4. Bulk BUKAN jalan kedua.** Satu berkas cuma daftar sepanjang satu: form yang
sama, tombol yang sama, antrian yang sama. Kegagalan satu video dicatat pada
video ITU lalu antriannya lanjut — 29 berkas tidak dibuang karena yang ke-30
tidak punya suara.

## Yang dipakai ulang, bukan dibuat ulang

- **Transkripsi** — `pipeline.Transcript`, dipisahkan dari `pipeline.Run` pada
  sesi ini. Tahapnya sama persis, termasuk cachenya: membuat caption untuk klip
  yang baru dipotong tidak mentranskripsi apa pun untuk kedua kalinya.
- **Ucapan sebagai teks** — `subtitle.Text`, dipisahkan dari `subtitle.WriteText`.
  Bentuknya sama dengan berkas `.txt` pendamping klip, jadi tidak ada dua
  gagasan berbeda tentang "ucapan tanpa waktu".
- **Mesin LLM** — `api.EngineFor` + `writer.Completer` + `<EnginePicker>`,
  daftar mesin yang sama untuk seluruh aplikasi (notes/39).
- **Pembaca balasan JSON** — `writer.ExtractJSON` & `writer.JSONError`, kini
  diekspor. Model yang tidak dipaksakan skemanya membalas larik telanjang atau
  dibungkus pagar kode; menolaknya berarti membuang pekerjaan yang isinya sudah
  benar.
- **Job latar + SSE** — `bgStore[T]` di `internal/api/bgjob.go`, hasil
  menggenerikkan mesin job pembuat berita. Pembuat berita dan pembuat caption
  memakai store yang sama, dibedakan hanya tipe hasilnya.
- **Daftar istilah** (`-terms`) — daftar yang sama dengan halaman klip. Nama
  daerah yang salah dengar jauh lebih terlihat di caption daripada di subtitle.

## Koreksi transkrip: sempat dilewati, dan itu kekeliruan terbesarnya

Awalnya tahap `correct` dilewati dengan alasan "mahal, caption tidak butuh
presisi kata-per-kata seperti subtitle". Hasil job pertama (20 klip, whisper
small, llama3.1) membantah itu telak:

```
transkrip : "Aku cuma waktu di luar negeri tuh kayak coba apa namanya Aikos."
caption   : "Roko doang aku gak suka kan"        ← salah dengar tersalin utuh
tagar     : #Aikos                                ← salah dengar jadi tagar
```

Caption ditulis DARI teks itu, jadi tiap salah dengar naik pangkat: dari satu
kata di subtitle yang lewat dalam dua detik, menjadi judul yang menempel di
postingan. Koreksi kini MENYALA secara bawaan, memakai mesin yang sama dengan
penulis captionnya (`-transcript-fix off` untuk mematikannya).

Ia juga yang membuat `-terms` berguna: whisper dipanggil dengan `-mc 0` sehingga
kosakatanya tidak bisa dibias saat transkripsi (CLAUDE.md), dan daftar istilah
baru berlaku di tahap koreksi. Klip yang sama, dijalankan ulang dengan
`-terms "IQOS,vape,shisha"`:

```
"roko" → "rokok" · "Aikos" → "IQOS" · "Sisha" → "shisha" · "rokot" → "merokok"
caption: "Bukan berhenti merokok total, tapi metode baru!"
```

Ongkosnya satu panggilan LLM lagi per video. Untuk klip pendek itu hitungan
detik — dan itu perbandingan yang salah kalau alternatifnya caption yang tidak
bisa dipakai.

## Tiga tuas kualitas, sesuai besarnya

1. **Model whisper.** Caption tidak bisa lebih baik daripada kata-kata yang
   dipakai menulisnya. Karena itu pemilihnya ada DI HALAMAN INI, bukan dianggap
   urusan pemasangan.
2. **Koreksi transkrip** (di atas).
3. **Model penulis.** llama3.1 8B tetap mengarang klaim yang tidak ada di
   videonya — "mencoba IQOS dan vape" padahal videonya menyebut vape justru
   tidak pernah dipakai. Pagar angka tidak menangkapnya sebab tidak ada angka
   di dalamnya. Yang menangkap: manusia yang memilih satu dari tiga, dengan
   transkrip di berkas yang sama.

Jumlah caption ikut dipaku di skema (`minItems` = jumlah yang diminta): tanpa
itu llama3.1 membalas 1-2 caption walau promptnya menyebut tiga.

## Yang BELUM ada

- **Koreksi transkrip tidak bisa dimatikan dari GUI** (CLI punya
  `-transcript-fix off`). Bawaannya menyala, dan mematikannya nyaris selalu
  keputusan yang salah.
- **Bahasa caption mengikuti bahasa video** (`id`), tidak bisa dipilih di GUI.
  Sasaran proyek konten Indonesia; CLI punya `-lang` bila perlu.
- **Tidak ada tombol buka-folder** di kartu hasil, seperti klip punya.

## Path dari Explorer: tiga bentuk, dan dua di antaranya sempat ditolak

Dilaporkan dari lapangan sebagai "error, videonya tidak ada" — padahal
videonya jelas ada. Sebabnya bentuk path yang ditempel, bukan berkasnya:

```
C:\Users\Budi\video.mp4                     diterima sejak awal
"C:\Users\Budi\video.mp4"                   DITOLAK — "Copy as path" Explorer
                                             SELALU memasang kutip
\\wsl.localhost\Ubuntu\home\budi\video.mp4   DITOLAK — bentuk yang dipakai
                                             Explorer untuk berkas sisi Linux
```

Ketiganya keluar dari satu perintah yang sama (Shift + klik kanan → Copy as
path), jadi menerima satu di antaranya saja berarti fitur ini gagal untuk cara
paling wajar orang menyalin alamat berkas di Windows.

Terjemahannya di `hostPath` (`internal/api/files.go`), dipanggil di SEMUA pintu
masuk yang menerima path dari klien — `/api/browse`, `/api/jobs`,
`/api/captions`, `/api/probe`, `/api/frame`, `/api/requirements/path` — bukan
hanya di yang kebetulan dilaporkan. Share jaringan sungguhan
(`\\server\bagi\...`) sengaja TIDAK diterjemahkan: ia bukan berkas mesin ini,
dan menebak terjemahannya cuma menghasilkan path palsu.
