# LLM memilih NOMOR, engine yang memegang angka waktu

Sebelumnya model diminta menulis sendiri `start` dan `end` momen. Itu meminta
model melakukan hal yang paling tidak bisa ia lakukan.

## Yang terukur sebelum perubahan

Tiga kali jalan, transkrip dan permintaan sama persis (target 30-60 detik, 3
klip, qwen2.5):

| | Hasil | Kemiripan antar-jalan |
| --- | --- | --- |
| temp 0,4 (bawaan) | `19-27`, `468-43`, `367-313` / `20-27` / `20-27`, `41-44`, `243-250` | 1,00 / **0,44** / **0,44** |
| temp ~0 | `19-27` tiga kali | 1,00 / 1,00 / 1,00 |

Dua hal terbaca dari situ:

- **`468-43` dan `367-313`: akhir lebih kecil dari awal.** Rentang mustahil.
- **`19-27` = 8 detik** padahal diminta 30-60.

Yang menyelamatkan selama ini `validateMoments`: rentang terbalik dibuang, di
luar durasi dibuang, terlalu pendek dibuang. Karena itu pengguna melihat "klipnya
aneh dan berubah-ubah", bukan error — sebagian besar balasan model memang dibuang
diam-diam, dan sisanya yang jadi klip.

**Menurunkan temperature bukan obatnya.** Ia memang membuat hasilnya bisa
diulang, tapi yang stabil itu satu momen 8 detik. Determinisme cuma mengubah
undian jadi jawaban salah yang konsisten.

## Perubahannya

Engine membangun kandidat dari batas kalimat (`segment.BuildCandidates`), lalu
mengirimkannya **bernomor**. Model hanya mengembalikan nomor + penilaian.

Seluruh kelas kegagalan itu hilang karena angkanya tidak pernah ada di tangan
model. Ini pola yang sudah terbukti di tab kartu berita: LLM hanya memilih nomor
paragraf, teksnya diambil engine, dan kartunya tidak pernah mengarang
(`notes/13`).

## Yang terukur sesudahnya

13 kandidat dari 600 detik pertama, semuanya berdurasi sah (43-62 detik):

| Jalan | Pilihan |
| --- | --- |
| 1 | `[4]`=85, `[10]`=82 |
| 2 | `[4]`=85, `[10]`=80 |
| 3 | `[4]`=85, `[3]`=75 |

Kandidat `[4]` terpilih di ketiga jalan dengan skor sama. Rentang mustahil tidak
mungkin lagi muncul — bukan karena model membaik, tapi karena tidak ada lagi
yang bisa ia karang.

Diuji end-to-end pada potongan 5 menit: 3 klip terender, batas waktunya semua
sah, judul & tagar terisi.

## Batch, bukan potongan transkrip

Dulu transkrip dipecah per 12 menit (Ollama) dan tiap potongan satu panggilan.
Sekarang kandidat dibangun dari SELURUH transkrip sekaligus lalu dikirim
berbatch 12 kandidat.

Bedanya bukan kosmetik: kandidat dipotong di batas kalimat, jadi tidak ada momen
yang terbelah di sambungan potongan. Seluruh mesin penyambungan lintas potongan
(`Moment.Continues`, `mergeMoments`, `chunkTranscript`, `chunkOverlap`) karena
itu ikut dicabut — bukan disimpan "kalau-kalau".

Batas 12 kandidat per permintaan bukan soal kemampuan model membaca melainkan
jendela konteksnya: `num_ctx` 8192 token, satu kandidat 100-200 kata.

## Yang ikut dibuang

Sesuai `notes/15` — begitu perbaikan yang benar masuk, tambalan dari tebakan
sebelumnya HARUS ikut dicabut:

`SelectMoments` (kedua mesin), `SystemPrompt`, `UserPrompt`, `ResponseSchema`,
`Moment`, `MomentsWrapper`, `Chunk`, `chunkTranscript`, `chunkSeconds`,
`mergeMoments`, `validateMoments`, `momentsToClips`, `snapToSegments`,
`fitDuration`, beserta tesnya.

Semuanya ada untuk merapikan angka karangan model. Tidak ada lagi angka karangan.

## Yang perlu diketahui

**Durasi klip sekarang mengikuti kandidat apa adanya**, tanpa `fitDuration`.
Pada target 48-75 detik, hasil uji: 43, 27, dan 78 detik. Itu sifat
`BuildCandidates` yang memang sudah ada — ia memotong di batas kalimat dan
menyimpan sisa ekor bila >= `targetMin*0.5`. Memaksanya masuk rentang berarti
memotong di tengah kalimat, persis yang dulu dihindari.

Kalau ekor pendek itu mengganggu, yang perlu diubah `targetMin*0.5` di
`BuildCandidates` — bukan menambahkan perapian setelahnya.

**Model masih menaruh tagar di dalam judul** ("Londohirang: Penghianat atau
Kritik? #Londohirang #Penghianat"), padahal prompt memintanya terpisah. Tidak
merusak apa pun, tapi belum dijaga.
