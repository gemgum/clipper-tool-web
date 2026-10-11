# Cuplikan pembuka: klip yang terdengar ramai tapi tidak membahas apa pun

Banyak video dibuka dengan cuplikan — potongan pendek dari sepanjang video yang
disambung jadi satu. Ia dirancang supaya terdengar seru, jadi mesin skor
menyukainya. Tapi ia berganti topik tiap beberapa detik dan tidak menuntaskan
apa pun, sehingga klip yang dipotong dari sana jadi trailer untuk video yang
tidak akan ditonton siapa pun.

Contoh nyata, `clip_01` sepanjang 48 detik yang memicu catatan ini — empat topik
tak berhubungan:

```
Universitas Republik Indonesia. Apa tanggapan Pak Mahfud...
Saya pernah juga loh dituduh Londo-Irang.
Ada para koruktor saya sikat saya kejar itu, anggota DPR...
3-4 hari lalu muncul berita meninggalnya seorang tukang jaga rumahnya Pak Febri
```

## Prompt saja tidak cukup

Aturan "jangan ambil momen dari cuplikan pembuka" ditambahkan ke
`llm.SystemPrompt` (dipakai Claude maupun Ollama). Diuji langsung ke qwen2.5
pada transkrip yang sama: **momen 41-57 detik tetap dipilih**.

Ini pola yang berulang di proyek ini — model lokal tidak terikat instruksi
panjang. Persis seperti waktu qwen2.5 membalas satu entri untuk 32 segmen sampai
dipaksa JSON Schema (`notes/14`). Yang mengikat adalah pagar, bukan kalimat.

## Sinyal yang GAGAL: ketidaknyambungan

Dugaan pertama: cuplikan bisa dikenali karena topiknya tidak nyambung. Diukur —
kesinambungan kosakata antar-paruh pada jendela 45 detik:

| Jendela | Kesinambungan |
| --- | --- |
| 0-45 (cuplikan) | 0,22 |
| 45-90 (cuplikan) | 0,07 |
| 90-135 (isi) | 0,00 |
| 225-270 (isi) | 0,00 |
| 630-675 (isi) | 0,00 |

Jendela isi justru lebih "tidak nyambung" daripada cuplikannya. Masuk akal:
percakapan spontan memang berpindah topik, dan 45 detik terlalu pendek untuk
mengulang kata kunci. Sinyalnya dibuang sebelum sempat jadi kode.

## Sinyal yang BEKERJA: cuplikan adalah pratayang

Yang khas dari cuplikan bukan ketidaknyambungannya, melainkan bahwa ia
**dipotong dari isi video**. Karena itu hampir seluruh kosakatanya muncul lagi
jauh di belakang.

Diukur pada lima transkrip sungguhan (kata >= 6 huruf, dibandingkan dengan
seluruh isi mulai 120 detik setelah jendelanya):

| | Gema ke isi berikutnya |
| --- | --- |
| Cuplikan (video bercuplikan) | **0,97 dan 0,95** |
| Seluruh jendela isi, semua video | 0,33 - **0,81** |
| 90 detik pertama, empat video tanpa cuplikan | 0,33 - 0,76 |

Ambang `previewThreshold = 0.90` diambil di antara 0,81 dan 0,95, condong ke
sisi aman: lebih baik satu cuplikan lolos daripada satu momen isi yang bagus
dibuang.

Diverifikasi ke lima transkrip: hanya video bercuplikan yang tertandai, dan
hanya di 90 detik pertamanya. **Nol salah tuduh di empat video lain.**

## Dua pembatas yang membuatnya aman

- **Hanya berlaku di 180 detik pertama** (`previewOpening`). Cuplikan selalu di
  pembuka; membatasi pemeriksaan ke sana membuat momen di tengah video mustahil
  salah tuduh, berapa pun kosakatanya bergema — di sana pengulangan kata justru
  tanda pembicaraan masih di topik yang sama.
- **Tidak pernah membuang habis.** Kalau semua momen tertandai, penyaringan
  dilewati: yang tersisa cuma pilihan buruk, dan pilihan buruk masih lebih baik
  daripada job gagal dengan "tidak ada klip".

## Yang BELUM dijaga

Ambang 0,90 diturunkan dari lima video, dan hanya satu di antaranya benar-benar
punya cuplikan. Satu contoh positif bukan sampel. Kalau nanti ada video
bercuplikan yang lolos, angka yang perlu diperiksa lebih dulu adalah
`previewGap` (120 detik) — cuplikan yang isinya dibahas ulang dalam 2 menit
pertama tidak akan terdeteksi.

Penyaringan ini juga tidak menyentuh alasan lain sebuah klip bisa jelek. Ia
hanya menjawab satu pola yang bisa dikenali tanpa memahami isinya.
