# Tiga cara memasang video ke bingkai 9:16

Ada TIGA pilihan yang berdiri sendiri, plus satu kendali zoom yang terpisah dari
ketiganya. Salah paham soal ini bikin fiturnya bongkar-pasang empat kali, jadi
istilahnya dikunci di sini.

## Ketiga mode (`reframe`)

| Nilai | Nama di layar | Perilaku |
| --- | --- | --- |
| `center` | **Center of the Picture** | potong tengah sampai memenuhi bingkai |
| `fit` | **Whole Picture** | ambil SELURUH resolusi video tanpa crop — seluruh gambar masuk |
| `face_follow` | **Follow Face** | potong mengikuti wajah (belum dibangun) |

**Whole Picture adalah alasan `background` ada.** Karena tidak ada yang dipotong,
video tidak mungkin memenuhi bingkai tegak; sisa ruangnya diisi blur dari
videonya sendiri atau hitam polos. Tanpa mode ini, pilihan latar tidak punya
alasan untuk eksis.

Nilai `fit` dipertahankan di kode & API karena itu istilah standar di dunia video
dan CSS. Nama "Whole Picture" dipilih agar sejajar dengan "Center of the
Picture" — dua nama, satu keluarga, langsung terbaca bedanya.

## Zoom dibaca relatif terhadap titik awal modenya

`zoom` bukan angka mutlak. Ia menjawab "seberapa jauh di-zoom DARI titik awal
mode ini", jadi batas bawah & nilai awalnya berbeda:

| mode | titik awal | arah yang tersedia | berhenti di |
| --- | --- | --- | --- |
| **Whole Picture** | 0 — seluruh video masuk | hanya NAIK | 100 = isi penuh |
| **Center of the Picture** | 100 — isi penuh | hanya TURUN | 5 = potongan mengecil |

Tiap mode mulai di ujung yang berlawanan, jadi masing-masing hanya punya SATU
arah yang masuk akal. Keduanya berhenti di 100: di situ gambar sudah memenuhi
bingkai, dan memperbesarnya lagi tidak menambah apa pun selain memotong lebih
banyak. Zoom 100 di WP menghasilkan gambar yang sama dengan COTP 100 — itu
konsekuensi geometri, bukan aturan yang ditulis di kode.

Di Whole Picture, menggeser dari 0 memang MEMOTONG sisi videonya. Itu bukan bug
melainkan yang diminta: mode ini memberi titik awal "semuanya masuk", lalu
pengguna memperbesar sesukanya dari sana.

Karena artinya berbeda, membawa angka zoom menyeberang saat mode berganti hanya
menghasilkan nilai yang tak berarti — GUI menyetelnya ke titik awal mode baru,
dan CLI memakai `flag.Visit` untuk membedakan "-zoom tidak disebut" dari
"-zoom disebut", supaya `-reframe fit` tanpa `-zoom` tidak justru menghasilkan
gambar terpotong penuh.

## Empat kali salah, dan apa penyebabnya

Fitur ini dibongkar-pasang empat kali sebelum kembali benar:

1. Zoom diubah jadi sumbu fit↔fill, dan `fit` dibuang dari dropdown. Akibatnya
   **Center of the Picture kehilangan artinya** — dilaporkan sebagai *"fitur
   9:16 fullnya hilang"*.
2. Saya kira yang hilang tampilan lain, lalu menambah sumbu kedua yang tidak
   pernah diminta.
3. Arah angkanya saya balik. Penggeser jadi terasa **tidak jalan sama sekali**.
4. Saat diminta agar WP bisa membesar, saya menaikkan langit-langit zoom ke 200
   alih-alih memindahkan titik awal WP ke 0. Itu tambalan di tempat yang salah.
5. Setelah titik awalnya dipindah ke 0 — perbaikan yang benar — langit-langit
   200 itu jadi sisa yang tidak dibuang, dan memunculkan "zoom 200%" yang tidak
   pernah ada di permintaan siapa pun.

Pelajaran dari nomor 4-5: begitu perbaikan yang benar masuk, tambalan dari
tebakan sebelumnya HARUS ikut dicabut. Membiarkannya menumpuk membuat fitur
punya perilaku yang tidak pernah diminta siapa pun.

Akar semua kesalahan itu satu: saya memperlakukan `reframe` dan `zoom` sebagai
dua cara menyatakan hal yang sama, lalu "menyederhanakannya" jadi satu sumbu.
Padahal keduanya menjawab pertanyaan berbeda — **cara memasang** vs **seberapa
besar**. Menggabungkannya membuang satu pertanyaan utuh.

## Jebakan yang ditemukan sepanjang jalan

**Parameter tak dikenal diabaikan diam-diam.** Saat `zoom` sempat berganti nama,
klien yang masih mengirim `?zoom=` tetap dibalas HTTP 200 — hanya saja nilainya
tidak berpengaruh. Gejalanya persis "tidak jalan sama sekali": tanpa galat, tanpa
perubahan gambar.

**Nilai nol yang punya arti.** `POST /api/jobs` kini menyemai `DefaultOptions()`
SEBELUM `json.Decode`. Decoder hanya menimpa field yang benar-benar ada di body,
jadi field yang tidak dikirim mempertahankan defaultnya. Satu baris, berlaku
untuk seluruh field.

**Dimensi ganjil di tahap antara itu wajar.** Pada Whole Picture, ffmpeg menjaga
rasio sumber sehingga hasil skalanya bisa ganjil (mis. 108x61). Itu aman: `pad`
sesudahnya selalu mengembalikannya ke ukuran bingkai yang genap, jadi h264 tidak
pernah melihatnya. Tes hanya menjamin kegenapan hasil AKHIR.

## Yang dijaga tes

- `TestCenterFullZoomFilterIsLockedDown` — rantai filter keluaran andalan
  dikunci persis sebagai string, jadi ia tidak bisa berubah diam-diam lagi.
- `TestWholePictureAlwaysHasBackground` — latar wajib hadir di Whole Picture
  pada zoom berapa pun, sebab itulah alasan mode ini ada.
- `TestWholePictureZoomGrowsWithoutCropping` — ffmpeg sungguhan dijalankan,
  ffprobe mengukur: tidak ada yang melebihi bingkai, dan tiap langkah membesar.
- `TestEveryFivePercentStepChangesSize` — tiap langkah 5% benar-benar mengubah
  ukuran, supaya penggeser tidak pernah lagi terasa mati.
- `TestWholePictureZoomsInAboveFullSize` — di atas 100% gambar wajib membesar,
  bukan mengecil; inilah keluhan terakhir yang diperbaiki.
- `TestCenterZoomAboveFullStillFillsFrameExactly` — punch-in di COTP tetap
  menghasilkan bingkai persis, tanpa perlu latar.
- `TestClipReframeFillsFrameInEveryMode` — render klip utuh di empat kombinasi
  mode & zoom; hasilnya wajib tepat 1080x1920, dan piksel tepi atas diperiksa
  untuk memastikan latarnya benar-benar terisi.
