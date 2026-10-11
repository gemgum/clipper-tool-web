# Koreksi transkrip oleh LLM

Masalah yang dilaporkan: subtitle klip "masih aneh bentuknya, ada tanda baca dan
strukturnya terlalu tidak masuk". Diperiksa di transkrip job nyata
(`data/cache/transcripts/d139…json`), dan keluhannya benar:

```
[ 85.72] - Dari kemarin harusnya digeledah. - Iya dong.     ← tanda hubung dialog
[105.88] ...berbicara jujur dan terus terang,               ← kalimat putus di koma
[113.32] apa adanya tanpa tedeng aling-aling.                  lalu nyambung di segmen lain
[138.48] ada beberapa poin yang lagi rime sekarang          ← salah dengar ("rame")
[151.28] Keluannya saja Pak, misalnya satu, Londo, Iran.    ← "Keluhannya"
```

Tanda hubung dialog itu konvensi subtitle whisper — ia ikut terbakar ke klip 9:16
dan terlihat salah. Tanda baca yang salah tempat lebih berbahaya lagi: 
`segment.BuildCandidates` memotong kandidat klip **di akhir kalimat**, jadi titik
yang salah letak menghasilkan batas klip yang salah pula.

## Keputusan

Tiga pilihan diajukan, tiga-tiganya dipilih ke sisi yang paling agresif:

| Pertanyaan | Pilihan |
| --- | --- |
| Cakupan | Termasuk memperbaiki kata salah dengar (bukan cuma tanda baca) |
| Dampak | Seluruh pipeline — segmentasi, scoring, dan subtitle |
| Kapan | Selalu jalan, termasuk saat mesin skornya heuristik |

Pilihan ketiga sempat disarankan untuk dihindari, lalu tetap dipilih. Artinya
mode "offline tanpa AI" kini tetap memanggil LLM untuk koreksi. Konsekuensinya
ditangani eksplisit, bukan disembunyikan: bila mesin koreksi tak terjangkau,
job **berhenti dengan pesan sebabnya** (sesuai notes/12), dan pesannya menyebut
cara mematikan koreksi.

Default menyala; saklarnya `-transcript-fix off` di CLI dan satu centang di GUI.

## Batas: mengoreksi, bukan menulis ulang

Risiko utama pilihan "termasuk kata salah dengar": model bisa mengarang koreksi,
dan itu tidak terdeteksi mesin. Empat pagar pengaman dipasang di
`acceptable()`, semuanya deterministik dan dijalankan atas SETIAP segmen:

1. **Balasan kosong** ditolak.
2. **Selisih jumlah kata** > 30% (minimal 3) ditolak — model memangkas atau
   menyisipkan kalimat.
3. **Jatah kata isi yang berubah**: maksimal satu per enam kata. Tanda hubung
   dialog tidak dihitung sebagai kata, sebab membuangnya justru tugas koreksi.
   Inilah pagar utamanya.
4. **Tanda kutip ganda yang hilang** ditolak. Kutip menandai kutipan langsung;
   membuangnya bukan koreksi. Pagar ini perlu terpisah karena perbandingan kata
   membuang tanda baca, jadi kutip yang hilang lolos tanpa terdeteksi.

Segmen yang koreksinya ditolak memakai teks aslinya, dan penolakannya **dihitung
serta dilaporkan** (`Report.Rejected`), bukan dibuang diam-diam.

## Timestamp per kata

Subtitle mode karaoke & word menyorot kata per kata memakai timestamp asli
whisper. Begitu teks berubah, pemetaan kata→waktu harus dibangun ulang atau
sorotannya meleset. `align.go` menyejajarkan deret kata lama & baru dengan jarak
edit level kata, lalu:

- kata yang cocok / diganti 1:1 → **timestamp asli dipakai apa adanya**;
- kata sisipan → membagi rata celah waktu di antara jangkar kiri & kanan;
- kata yang hilang → waktunya diserap kata sebelumnya, supaya sorotan tidak
  padam selagi audionya masih berbunyi.

Substitusi 1:1 adalah bentuk terbanyak koreksi ASR ("rime" → "rame"), dan justru
kasus itulah yang timing-nya terjaga sempurna.

## Temuan dari uji nyata (qwen2.5 7B, 32 segmen)

Dua hal ditemukan hanya karena dijalankan sungguhan, bukan dari membaca kode:

**1. Skema `minItems: 1` membuat model berhenti setelah satu entri.** Permintaan
berisi 32 segmen dibalas 1 entri; 31 segmen tidak pernah terkoreksi dan hanya
terlihat dari `Report.Missing`. Perbaikannya: `SegmentsSchema(n)` mengunci
`minItems` **dan** `maxItems` ke jumlah segmen yang dikirim, plus potongan
diperkecil ke 12 segmen. Prompt saja tidak cukup — batasannya harus dijamin di
sisi decoder. Ini pengulangan masalah yang sama dengan `"peringkat": []` di
notes/12.

**2. Suhu 0.4 terlalu tinggi untuk tugas menyalin-ulang.** Hasil berbeda tiap
kali dijalankan atas transkrip yang sama. Koreksi kini memakai 0.1 lewat field
`Temperature` baru di klien Ollama & Claude (0 = pakai bawaan, jadi pemilihan
momen tidak berubah perilakunya).

Hasil akhir pada 32 segmen itu: **12 segmen dikoreksi, 1 ditolak** — dan yang
ditolak persis kasus `"Londo-Irang" → "Londo-I rang"` sekaligus kata
`"Menyakitkan."` hilang. Pagar pengaman menangkap apa yang memang dibangun untuk
ditangkap.

## Batas kemampuan yang tersisa

qwen2.5 7B **tidak konsisten** memperbaiki kata salah dengar: `rime` → `rame`
kena di satu run, tidak di run lain. Tanda hubung dialog dan tanda baca jauh
lebih andal. Model yang lebih kuat (Claude di mode hybrid, atau model lokal yang
lebih besar) mengerjakan bagian salah dengar dengan lebih baik.

Biaya: ±30 detik per 32 segmen dengan qwen2.5 di CPU, jadi transkrip 439 segmen
(podcast satu jam) ≈ 7–8 menit. Hasilnya di-cache di `data/cache/corrected`
dengan kunci = sidik jari transkrip + nama mesin + versi prompt, jadi hanya
dibayar sekali per video.

## Uji manual

```bash
CLIPPER_TEST_TRANSCRIPT=data/cache/transcripts/<kunci>.json \
CLIPPER_TEST_SEGMENTS=32 \
  go test ./internal/correct/ -run TestCorrectLive -v
```

Mencetak setiap segmen yang berubah (sebelum/sesudah), dan memeriksa bahwa batas
segmen tidak bergeser serta kata bertimestamp tetap urut naik.

## Prompt v4 — MENUNGGU PENGUKURAN (4 Agustus 2026)

Butir terakhir "WHAT NOT TO TOUCH" dipertajam, `PromptVersion` naik ke `v4`.

**Sebabnya.** Butir lama berbunyi "any word you do not recognise at all …
write it exactly as it is". Terlalu lebar: ia ikut melindungi salah dengar kata
Indonesia biasa. Terlihat pada `job_0002_2026-08-04`, empat dari lima kesalahan
adalah kata umum, bukan nama:

| Tertulis | Seharusnya |
| --- | --- |
| terlalu gegab | gegabah |
| perguruan tinggi itu gagab | gagap |
| numenkelaturnya | nomenklaturnya |
| bukan rondo hirang | Londo Ireng (ini memang butuh `-terms`) |

**Pembedanya bukan kenal/tidak kenal, melainkan ada/tidaknya tetangga dekat.**
`ireng` tidak punya kata Indonesia yang berjarak satu-dua huruf (`irang` juga
bukan kata) → biarkan. `numenkelatur` berjarak dua huruf dari `nomenklatur` dan
kalimatnya jadi masuk akal → perbaiki. Butir baru memakai dua syarat itu
sekaligus, dan dua syarat itu WAJIB digabung: `mangan` berjarak dua huruf dari
`makan` DAN kalimatnya tetap masuk akal, jadi yang menahannya cuma rujukan
silang ke aturan bahasa daerah di atasnya.

**Sudah diuji:** `CLIPPER_TEST_LIVE=1 go test ./internal/correct/ -run Javanese`
lolos — kelima kata daerah utuh, termasuk `mangan` yang paling berisiko.

**Belum diukur:** apakah keempat kesalahan di tabel benar-benar hilang, dan
apakah `N correction(s) rejected as rewrites` ikut naik. Kenaikan angka tolakan
adalah efek samping yang paling mungkin: prompt yang lebih tajam membuat model
menyunting lebih banyak kata per segmen, dan begitu lewat jatah
`editBudgetDivisor = 6`, SELURUH koreksi segmen itu dibuang — termasuk
perbaikan tanda bacanya.

Cara mengukur: jalankan video yang sama seperti `job_0002`, sekali **tanpa**
`-terms` dulu supaya efek prompt terpisah dari efek daftar istilah.
