# Flag whisper: halusinasi, konteks, dan pilihan model

Catatan dari sesi 2–3 Agustus 2026. Isinya temuan yang mahal ditemukan ulang
kalau hilang.

## Masalahnya: whisper mengulang satu kalimat sampai audio habis

Job pertama dengan `large-v3` menghasilkan 6 klip, dan 5 di antaranya isinya
cuma satu kalimat diulang terus. Ditelusuri ke transkrip mentahnya:

```
84.34 → 85.84  " Seharusnya sudah digeledah."
86.50 → 87.00  " Iya dong."
89.06 → 90.06  " Terima kasih Pak Febri."   ← mulai mengunci
90.06 → 91.06  " Terima kasih Pak Febri."
...            (sampai menit ke-38)
```

**1603 dari 1644 segmen (97,5%)** isinya kalimat itu. Dari video 38 menit, yang
benar-benar tertranskrip hanya 89 detik pertama.

Pemicunya bumper acara di detik 89 — ada jingle di situ, dan bagian musik/hening
memang paling rawan.

## Sebabnya: konteks teks yang menguatkan diri sendiri

Decoder whisper itu model bahasa autoregresif. Untuk jendela 30 detik kedua dan
seterusnya, teks hasil jendela sebelumnya disuapkan balik sebagai konteks.

Begitu model tergelincir sekali, kalimat sampahnya ikut jadi konteks jendela
berikutnya, dan loop-nya menguatkan diri sendiri sampai audio habis. Tidak
pernah pulih.

Batas konteks itu diatur flag `-mc` (`--max-context`). Bawaan whisper-cli `-1`
artinya "jangan timpa", sehingga terpakai nilai pustaka 16384 — praktis tanpa
batas.

## Obatnya

Dua flag ditambahkan di `engine/internal/transcribe/whisper.go`:

- **`-mc 0`** — jangan suapkan teks sebelumnya sebagai konteks. Ini obat
  utamanya. Kerugiannya kecil untuk kita: subtitle dinilai per kalimat, bukan
  sebagai prosa panjang.
- **`-sns`** — tekan token non-ucapan (`[MUSIC]`, tepuk tangan), pemicu
  halusinasi di bagian hening.

Diuji langsung pada potongan 60–180 detik yang tadinya rusak: di detik 90 yang
dulu mengunci selamanya, keluar isi asli — *"Halo saudara semua, senang bisa
menyapa dan bertemu kembali di Terus Terang"*. 29 segmen, deretan identik
terpanjang = 1.

Flag `-et` dan `-lpt` sempat dipertimbangkan tapi tidak dipakai: nilainya persis
sama dengan bawaan, jadi hanya jadi baris kosong.

## Jebakan besar: `-mc 0` mematikan `--prompt`

Ini temuan yang paling mudah terlupa dan paling mahal ditemukan ulang.

Whisper punya `--prompt` untuk menitipkan kosakata (misalnya nama orang atau
istilah daerah) supaya decoder condong ke ejaan yang benar. Tapi di
`third_party/whisper.cpp/src/whisper.cpp` baris 7111:

```cpp
if (params.n_max_text_ctx > 0 && t_cur < WHISPER_HISTORY_CONDITIONING_TEMP_CUTOFF) {
    const bool can_take0 = params.carry_initial_prompt && !prompt_past0.empty();
```

Blok yang menyuntikkan initial prompt ada **di dalam** penjagaan
`n_max_text_ctx > 0`. Jadi dengan `-mc 0`, jalur biasing kosakata ikut mati.

Dibuktikan dengan tiga kali jalan pada potongan 30 detik yang sama:

| | flag | hasil |
| --- | --- | --- |
| A | `-mc 0` + prompt | "Londo Hirang" — **identik dengan C** |
| B | `-mc 224` + prompt | **"Londo Ireng" ×3** — benar |
| C | `-mc 0` tanpa prompt | "Londo Hirang" |

A sama persis dengan C, artinya prompt-nya benar-benar diabaikan.

Konsekuensinya praktis biner: **biasing kosakata nyala berarti risiko loop balik
lagi**, karena `max_prompt_ctx = min(mc, 224)` — jadi `-mc 224` itu sama saja
dengan perilaku bawaan yang bikin loop.

Prompt juga bukan tanpa efek samping: di percobaan B, "Mahfud" justru jadi
"Mahfu" padahal tanpa prompt sudah benar.

Karena itu kosakata dibenahi di tahap koreksi, bukan di decoding. Lihat
`notes/22-daftar-istilah.md`.

Kalau suatu saat mau dua-duanya, jalannya menambal satu syarat di whisper.cpp
yang di-vendor supaya `--carry-initial-prompt` tetap hidup saat `-mc 0` — plus
menjaga `n_take1` di baris 7127 tidak jadi negatif kalau prompt lebih panjang
dari `max_prompt_ctx`.

## Penjaga: detektor loop

Flag saja tidak cukup — kalau suatu saat lolos lagi, hasilnya diam-diam jadi
klip sampah seperti semula. Ditambahkan `engine/internal/pipeline/repetition.go`
yang menghentikan job sebelum tahap koreksi, dengan pesan yang menyebut kalimat
pengulangnya.

Dua sinyal dipakai:

- porsi segmen berteks sama > 50%
- deretan identik berturut-turut ≥ 30

Sinyal kedua penting. Waktu detektor ini diadu dengan cache lama, ketahuan video
"Sierra prestasi" (4 jam) juga kena: `[sampai jumpa di video selanjutnya]`
diulang **856 kali berturut-turut**, menutupi **31 menit pertama**. Porsinya
cuma 22,6% dari keseluruhan — di bawah ambang 50%, jadi yang menangkapnya justru
aturan deretan.

Diletakkan sebelum koreksi karena mengirim ribuan segmen sampah ke LLM memakan
20 menit dan hasilnya tetap sampah.

## Kunci cache ikut versi flag

Flag whisper tidak masuk kunci cache transkrip. Tanpa penanda versi, mengubah
flag lalu menjalankan ulang video yang sama akan memungut transkrip lama dan
perbaikannya seolah tidak berpengaruh.

Karena itu awalan versi di `transcriptCacheKey` dinaikkan `v1` → `v2`. **Naikkan
lagi setiap kali flag decoding berubah.**

## large-v3 vs large-v3-turbo untuk bahasa Indonesia

Diukur pada video yang sama, keluaran mentah sebelum koreksi:

| turbo | | large-v3 | |
| --- | --- | --- | --- |
| `Londoireng` | 17 | `Irang` | 21 |
| `Londo Ereng` | 4 | `Ireng` | 1 |
| `Londo Iireng` | 2 | | |
| `rondo hirang` | 1 | | |

Turbo mengeluarkan bunyi "ireng/ereng" di 23 dari ~33 kemunculan; large-v3
hampir tidak pernah. Kesan awal bahwa turbo "lebih kacau" datang dari satu
kemunculan `rondo hirang` — satu dari 33.

Masalah utama turbo bukan salah dengar, tapi **spasi hilang** (`Londoireng`),
dan itu justru perbaikan paling sepele untuk tahap koreksi.

Turbo juga jauh lebih ringan: berkasnya 1,5 GB vs 2,9 GB. Untuk mesin dengan
VRAM terbatas atau CPU ARM, itu bukan sekadar soal kecepatan.

Catatan risiko: kecepatan turbo datang dari decoder yang dipangkas (4 lapis,
dari 32) — dan decoder itulah yang terjebak loop. Detektor di atas jadi
pengamannya.
