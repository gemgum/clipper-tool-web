# Daftar istilah: memperbaiki nama & kata daerah

Catatan dari sesi 2–3 Agustus 2026.

## Masalahnya

Whisper konsisten salah menuliskan istilah Jawa "Londo Ireng" — jadi "Londo
Irang", "Londo Hirang", "Lendawirang", dan belasan variasi lain.

Naik ke `large-v3` tidak menolong sama sekali. Sebabnya bukan ukuran model, tapi
*language prior*: kita memanggil whisper dengan `-l id`, sementara "ireng" itu
bahasa Jawa. Decoder memetakan bunyinya ke rangkaian token yang paling masuk akal
sebagai bahasa Indonesia. Model lebih besar berarti kapasitas lebih besar dengan
prior yang sama.

Perhatikan sisa transkripnya justru bagus — "sejarah pengkhianatan terhadap
perjuangan", "Universitas Republik Indonesia" semuanya benar. Yang jebol hanya
idiom daerah dan nama orang. Ini masalah **kosakata**, bukan mutu engine.

## Kenapa tidak dibetulkan di whisper

Satu-satunya tuas di dalam whisper adalah `--prompt`, dan itu mati karena kita
memakai `-mc 0` untuk mencegah loop halusinasi. Selengkapnya di
`notes/21-flag-whisper.md`.

Jadi perbaikannya dipindah ke tahap koreksi transkrip, yang memang sudah ada.

## Kenapa tahap koreksi tidak menyelamatkannya sendiri

Ini bagian yang paling tidak terduga. Prompt koreksi **melarang** perbaikan itu.
Ada aturan di `correct.go`:

> Words from a REGIONAL LANGUAGE… do not "fix" their spelling towards a word you
> recognise. For example, the Javanese "ireng" must stay "ireng" — it is not a
> misheard "irang".

Aturan itu ditulis untuk **melindungi** "ireng" kalau whisper sudah benar. Tapi
arahnya kebalikan dari yang dibutuhkan: whisper mengeluarkan "Irang", dan prompt
menyuruh LLM membiarkan kata asing apa adanya. Jadi salah dengarnya lolos
**sesuai desain**.

Pelajarannya: pagar pengaman yang dipasang untuk satu arah bisa memblokir arah
sebaliknya. Kalau menambah aturan "jangan sentuh X", pikirkan juga kapan X
justru harus disentuh.

## Bentuk fiturnya

Pengguna mengisi daftar ejaan baku yang dipakai di video itu — misalnya
`Londo Ireng, Mahfud MD, URI`.

- di GUI: kolom di bawah centang "Perbaiki transkrip dengan AI"
- di CLI: `-terms "Londo Ireng,Mahfud MD,URI"`

Pemisahnya koma, titik koma, atau baris baru. Dibatasi 40 istilah — daftar yang
lebih panjang dari itu menggeser perhatian model dari transkripnya sendiri.

Daftar itu disuntikkan ke prompt sistem sebagai blok **pengecualian** yang tegas
menimpa dua aturan "jangan sentuh" di atas — tapi hanya untuk kecocokan bunyi
yang dekat.

## Tiga hal yang tidak kelihatan tapi menentukan

**1. Pagar pengaman bisa menolak perbaikannya sendiri.** Jatah perubahan per
segmen adalah satu kata isi per enam kata. Segmen pendek berjatah satu, dan
jatah itu sering sudah habis oleh pembenahan tanda baca. Karena itu perubahan
yang *menuju* ejaan di daftar tidak lagi memakan jatah — ejaannya sudah
disetujui pengguna, jadi itu koreksi terarah, bukan karangan. Penulisan ulang
tetap ditolak; ada test yang menjaga.

**2. Kunci cache wajib ikut daftar istilah.** Tanpa itu, menambah satu istilah
lalu menjalankan ulang video yang sama akan memungut hasil koreksi lama dan
perbaikannya seolah tidak berefek. `PromptVersion` juga dinaikkan ke `v3`.

**3. Satu istilah salah dengan beberapa cara berbeda dalam satu video.** Prompt
harus menyebutkan itu secara eksplisit — bentuk berhubung (`Londo-Irang`), cuma
satu katanya yang salah (`Londo Iram`), atau berdiri sendiri tanpa kata
pasangannya (`Anda yang Irang`). Sebelum ditambahkan, model hanya membetulkan
bentuk yang paling umum.

## Hasil pengukuran

Diuji pada 9 segmen sungguhan dari transkrip, dua arah:

| | masih salah | sudah benar |
| --- | --- | --- |
| tanpa daftar (kontrol) | 8 | 0 |
| dengan daftar, qwen2.5 | 2 | 6 |
| dengan daftar, llama3.1 | **1** | **7** |

**llama3.1 jelas lebih baik dari qwen2.5** untuk tugas ini — qwen2.5 juga gagal
membuang tanda hubung dialog Unicode. Untuk koreksi transkrip, pilih llama3.1.

Yang tersisa satu: bentuk berhubung `Londo-Irang`.

## Jebakan pemakaian

Dua job berturut-turut berjalan **tanpa** daftar istilah tanpa disadari, karena
kolomnya kosong saat Start ditekan. Gejalanya membingungkan: fiturnya seperti
tidak bekerja.

Karena itu baris progres koreksi sekarang menyebut jumlah istilah:

```
Ollama (llama3.1) is correcting the transcript (3 terms)
```

Kalau tertulis tanpa `(3 terms)`, daftarnya tidak sampai. Ketahuan saat itu
juga, tanpa perlu membongkar berkas cache.

Cara membuktikannya dari cache kalau perlu: nama berkas di `data/cache/corrected/`
adalah sha256 dari `kunci transkrip + nama mesin + versi`. Versi `v3` polos
berarti tanpa istilah; `v3|terms:...` berarti terkirim.

## Sekalian dibetulkan

Aturan penghapus tanda hubung dialog hanya mencontohkan `-` ASCII, sedangkan
whisper juga mengeluarkan `−` (minus) dan `—` (em dash). Ketiganya sekarang
disebut eksplisit di prompt.
