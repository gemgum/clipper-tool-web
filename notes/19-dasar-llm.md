# Dasar LLM, dijelaskan lewat proyek ini

Catatan belajar. Semua contohnya diambil dari kejadian nyata di Clipper, bukan
contoh buatan — supaya istilahnya menempel pada sesuatu yang bisa dilihat.

## 1. Yang sebenarnya dilakukan LLM

LLM tidak "memahami lalu menjawab". Ia melakukan satu hal berulang-ulang:
**menebak potongan kata berikutnya.**

Diberi `"Ibu kota Indonesia adalah"`, ia menghitung probabilitas tiap kemungkinan
lanjutan:

```
" Jakarta"  62%
" DKI"      11%
" kota"      4%
" sebuah"    2%
... ribuan kemungkinan lain
```

Lalu memilih satu, menempelkannya, dan mengulang dari awal dengan teks yang sudah
lebih panjang. Jawaban sepanjang paragraf adalah ratusan tebakan berurutan.

Ini akar hampir semua yang kita temui: model tidak punya niat, tidak punya
ingatan antar-panggilan, dan tidak "tahu" bahwa akhir harus lebih besar dari
awal. Ia menebak angka yang *terlihat masuk akal sebagai kelanjutan teks*.

## 2. Token

Model tidak melihat huruf atau kata, tapi **token** — potongan kata.
`"mengatakan"` bisa jadi `meng` + `atakan`. Kira-kira 1 token ≈ 3-4 huruf untuk
bahasa Indonesia.

Bukan detail akademis: semua batas di proyek ini dihitung dalam token.

## 3. Jendela konteks (`num_ctx`)

Batas berapa token yang bisa dilihat model sekaligus, mencakup **masukan +
keluaran**.

| | Nilai |
| --- | --- |
| Kemampuan maksimum qwen2.5 | 32.768 token |
| Yang diminta engine (`numCtx` di `ollama.go`) | **8.192 token** |

Jendela besar memakan RAM dan memperlambat, jadi kita minta secukupnya. 8.192
token ≈ 25-30 ribu huruf ≈ 4-5 ribu kata.

**Dari sinilah batas 12 kandidat per permintaan** (`MaxCandidatesPerRequest`):
12 × ~150 kata ≈ 3.000 token, ditambah instruksi ~500, ditambah ruang balasan
`num_predict` 2.048. Kalau dikirim 30 kandidat, kandidat pertama terdorong
keluar jendela — model menilai sesuatu yang tidak lagi dilihatnya, dan itu tidak
menghasilkan error, cuma jawaban ngawur.

## 4. Bagaimana satu token dipilih — di sinilah temperature

Model menghasilkan daftar probabilitas. **Cara memilih dari daftar itu bukan
bagian dari modelnya** — itu setelan kita.

**Temperature** mengubah setajam apa perbedaan probabilitasnya:

| Temperature | Efek |
| --- | --- |
| 0 | selalu ambil yang tertinggi (*greedy*) — sama input, sama output |
| 0,4 (bawaan proyek ini) | biasanya yang tertinggi, kadang yang kedua/ketiga |
| 1,0 (bawaan Claude) | mengikuti probabilitas apa adanya |
| > 1 | perbedaannya diratakan, makin acak |

Terukur di proyek ini (tiga kali jalan, masukan sama persis):

| Temperature | Kemiripan antar-jalan |
| --- | --- |
| 0,4 | 1,00 / **0,44** / **0,44** |
| ~0 | 1,00 / 1,00 / 1,00 |

Klip yang berubah-ubah tiap kali dijalankan bukan karena model "berubah
pikiran" — kita memang menyuruhnya mengundi.

Istilah tetangganya:

- **top_k** — hanya pertimbangkan k token teratas. `top_k=1` sama dengan greedy.
- **top_p** (*nucleus sampling*) — pertimbangkan token teratas sampai
  probabilitas kumulatifnya mencapai p; membuang ekor panjang yang aneh.
- **seed** — angka awal pengundian. Seed sama + temperature sama = hasil sama
  persis. Belum dipakai di proyek ini.

## 5. `num_predict` / `max_tokens`

Batas panjang balasan (2.048 di `Complete`). Kalau model butuh lebih, kalimatnya
**terpotong di tengah** — dan kalau yang terpotong itu JSON, hasilnya JSON rusak.
Sebagian kegagalan "balasan tidak terbaca" berasal dari sini, bukan dari model
yang bodoh.

## 6. System prompt vs user prompt

- **System** — aturan main: "kamu memilih klip, balas JSON".
- **User** — bahan yang dikerjakan: daftar kandidatnya.

Pemisahan ini konvensi, bukan hukum. Model membaca keduanya sebagai satu untai
teks; system diberi bobot lebih oleh cara model dilatih.

## 7. Prompt bukan jaminan — pelajaran terpenting proyek ini

Instruksi di prompt hanyalah teks yang **menggeser probabilitas**. Ia tidak
memaksa apa pun. Bukti terukur di proyek ini:

| Yang ditulis di prompt | Yang terjadi |
| --- | --- |
| "jangan ambil dari cuplikan pembuka" | tetap memilih 41-57 detik |
| "pertahankan kata daerah" | `warnane` → `warna` |
| "jangan di bawah 18 detik" | mengembalikan momen 4 detik |
| "balas satu entri per segmen" | 1 dari 32 entri |

Yang benar-benar mengikat ada tiga jenis, ketiganya dipakai di sini:

1. **JSON Schema** (`format` di Ollama) — bekerja di tingkat pemilihan token:
   token yang akan merusak bentuk JSON diberi probabilitas nol. Model **tidak
   bisa** membalas di luar bentuk itu.
2. **Pagar di kode** — `acceptable()` di koreksi transkrip, `picksToClips()`
   yang membuang nomor di luar daftar.
3. **Merancang ulang tugasnya** — model tidak lagi diminta mengarang angka
   waktu, jadi angka waktu ngawur tidak mungkin ada.

Nomor 3 selalu paling kuat: bukan menangkap kesalahan, melainkan membuat
kesalahannya tidak bisa terjadi.

## 8. Halusinasi

Model mengarang dengan percaya diri karena ia mengoptimalkan "kelanjutan yang
masuk akal", bukan "kebenaran". `468-43` bukan kerusakan — bagi model itu dua
angka yang wajar muncul di posisi itu.

Karena itu kaidah di tab kartu berita ("LLM hanya memilih nomor paragraf,
teksnya diambil engine") bukan gaya melainkan pertahanan: **kalau model tidak
pernah menulis datanya, ia tidak bisa memalsukannya.**

## 9. Ukuran model & kuantisasi

`qwen2.5` yang dipakai: **7,6 miliar parameter**, kuantisasi **Q4_K_M**, 4,7 GB.

- **Parameter** = jumlah "kenop" hasil pelatihan. Makin banyak, makin mampu, dan
  makin berat.
- **Kuantisasi** = ketelitian penyimpanan tiap kenop. Aslinya 16 bit; Q4
  menyimpannya ~4 bit. Ukuran turun ~4×, bisa jalan di CPU biasa, tapi
  ketelitiannya berkurang — dan untuk mengikuti instruksi rumit, penurunan itu
  terasa.

Claude berukuran ratusan miliar parameter tanpa kuantisasi agresif. Jadi ketika
qwen2.5 mengabaikan aturan yang dipatuhi Claude, itu wajar — bukan salah setelan.

## 10. Melatih vs mengarahkan

Tiga hal yang sering tertukar:

| Cara | Apa yang berubah | Ongkos |
| --- | --- | --- |
| **Prompting** | tidak ada; cuma teks masukan | gratis, tidak mengikat |
| **Fine-tuning** | kenop modelnya benar-benar diubah | ribuan contoh + GPU, berjam-jam |
| **RAG** | tidak ada; bahan rujukan disisipkan ke prompt | murah, tapi tetap prompting |

"Melatih Ollama supaya paham bahasa Jawa" berarti fine-tuning, dan itu di luar
jangkauan proyek ini. Tapi masalah `ireng` → `irang` **tidak butuh model yang
tahu bahasa Jawa** — ia butuh model yang tidak mengubah kata yang tidak
dikenalnya. Itu soal pagar, bukan soal pengetahuan.

## Rujukan silang

- `notes/14-koreksi-transkrip.md` — JSON Schema memaksa satu entri per segmen
- `notes/13-kartu-berita.md` — LLM memilih nomor paragraf, engine menyediakan teks
- `notes/18-llm-memilih-nomor.md` — pola yang sama diterapkan ke pemilihan klip
- `notes/12-kebijakan-mesin-skor.md` — tanpa fallback diam-diam antar mesin
