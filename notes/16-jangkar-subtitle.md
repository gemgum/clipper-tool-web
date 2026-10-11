# Jangkar subtitle: titik yang digeser = tepi ATAS blok

Pengguna menaruh subtitle lewat preview, merasa sudah pas, lalu hasil rendernya
meleset begitu satu halaman subtitle berisi dua baris. Ada dua sebab terpisah,
dan keduanya harus diperbaiki bersama — memperbaiki satu saja tidak cukup.

## Sebab 1 — jangkar ada di tengah blok (`\an5`)

Dengan `\an5`, titik `\pos(x,y)` adalah **tengah** blok teks. Halaman 2 baris
jadi mengangkang titik itu: setengah naik, setengah turun. Akibatnya baris
pertama pindah tempat hanya karena jumlah barisnya berubah, dan subtitle
terlihat melompat-lompat sepanjang klip.

Diukur dari render libass sungguhan (font 72, jangkar `y=1500`, latar hitam,
baris piksel teratas yang berisi teks):

| | 1 baris | 2 baris | pergeseran baris pertama |
| --- | --- | --- | --- |
| `\an5` (lama) | 1474 | 1438 | **naik 36 px** |
| `\an8` (sekarang) | 1510 | 1510 | **0 px** |

Dipilih `\an8` (tepi atas), bukan `\an2` (tepi bawah): dengan tepi atas, titik
itu berarti "di sini baris pertama mulai" dan baris tambahan turun mengikuti
arah baca. Dengan tepi bawah, baris pertama justru naik makin jauh — persis
keluhan yang mau dihilangkan.

Konsekuensi yang disengaja: arti `subtitle.y` berubah dari "tengah blok" jadi
"atas blok". Preset lama akan turun kira-kira setengah tinggi blok, jadi posisi
perlu dicek ulang sekali.

## Sebab 2 — preview berbohong

Preview selalu menampilkan **satu** baris pendek, padahal `Pacing()` di engine
mengizinkan 2 baris (lambat/normal) atau 3 baris (padat). Pengguna memposisikan
sesuatu yang tidak mewakili hasilnya.

Sekarang preview menampilkan sebanyak baris terbanyak yang mungkin muncul, dan
jumlah itu dikunci di JSX. `white-space: nowrap` sengaja dipertahankan: kalau
CSS boleh melipat sendiri, jumlah baris di preview berubah diam-diam dan
preview berbohong lagi dengan cara yang baru — ini sempat terjadi saat
memperbaikinya. Baris contoh karena itu dijaga <= 19 karakter (batas satu baris
di engine pada ukuran font bawaan sekitar 22).

## Angka yang menyatukan keduanya

`line-height: 1.17` di CSS bukan tebakan — jarak antarbaris libass yang terukur
adalah 84 px pada ukuran font 72 (84/72 = 1,167).

Tinggi blok di GUI (`blockH = size * 1.17 * maxLines`) sedikit lebih besar dari
tinta sebenarnya, dan itu memang disengaja: ia dipakai untuk menyisakan ruang
di atas zona bawah, jadi kelebihan lebih aman daripada kekurangan.

`blockH` dipakai di tiga tempat, semuanya karena blok tumbuh ke bawah:

- **batas geser** — yang harus tetap di dalam bingkai adalah seluruh blok,
  bukan titik jangkarnya;
- **peringatan zona tidak aman** — kalau hanya jangkar yang diperiksa, baris
  kedua bisa menjulur ke zona caption sementara peringatannya diam saja;
- **magnet garis tengah** — supaya blok terlihat di tengah, jangkarnya harus
  setengah tinggi blok di atas garis itu.
