# Kolom kanan tergulir MENDATAR — 7 Agustus 2026

Dilaporkan dari laptop orang lain (potret layar, build Windows): halaman klip
terlihat runtuh — tiap baris di kolom kanan kehilangan kendali kolom
pertamanya, tombol Start tinggal `…g…`, sementara sisi kirinya (pratinjau,
subtitle, PROCESS LOG) baik-baik saja.

**Belum ditambal.** Catatan ini merekam sebabnya; suntikannya ada di bagian
terakhir.

## Rantai kejadiannya

**1. Teks tahap yang panjang.** Panel Start menampilkan
`${stage} — ${message} (${pct}%)`. Di mesin itu `message`-nya pesan Ollama untuk
pengguna Windows (`notes/34`):

```
correcting: Ollama (qwen3.5:latest) at http://localhost:11434 — Windows,
on this computer is correcting the transcript — part 4/183 (48%)
```

±135 karakter. Pesan kita sehari-hari (`transcribing: 100%`) tidak pernah
sepanjang itu, jadi bahan bakunya tidak pernah muncul di sini.

**2. Elipsisnya tidak punya batas untuk dipatuhi.** `.stage` sudah `nowrap` +
`overflow:hidden` + `text-overflow:ellipsis`, tapi elipsis hanya bekerja kalau
kotaknya punya lebar terbatas. Lebar itu datang dari induknya `.run-progress`,
dan induknya flex item — yang default-nya `min-width: auto`, artinya **selebar
min-content isinya**. Min-content teks `nowrap` = seluruh kalimat.

```
.run-progress  flex: 1 0 100%   → maunya 352 px
               min-width: auto  → lantainya 904 px (selebar kalimat)
menang: 904 px
```

**3. `overflow-y:auto` diam-diam menghidupkan sumbu X.** `.start-panel` ikut
melar jadi 920 px di kolom selebar 394. `.screen-col` punya `overflow-y: auto`,
dan begitu satu sumbu bukan `visible`, sumbu satunya yang `visible` dipaksa jadi
`auto` — kolom itu jadi bisa digulir mendatar tanpa pernah kita minta.

**4. Rupanya "hancur" karena kotak gulir memotong isinya.** Begitu kolom
tergulir ke kanan (roda mendatar, atau Chromium menggeser sendiri kendali yang
difokus lewat `scrollIntoView`), seluruh isi kolom bergeser bersama dan bagian
kirinya lenyap di balik tepi kotak. Sisi kiri tidak berubah sebab itu sel grid
yang lain. Kombinasi "kiri normal, kanan tergeser & terpotong" itu yang terbaca
sebagai tata letak yang runtuh.

## Angkanya (1240×765, diukur lewat CDP)

```
teks pendek : .screen-col cw=394 sw=394  over=0
teks panjang: .screen-col cw=394 sw=921  over=527
+ min-width:0 pada .run-progress:  .screen-col cw=394 sw=394  over=0
```

Setelah ditambal, `.stage` sendiri tetap `over=552` — itu memang elipsisnya
bekerja (isinya dipotong, kotaknya tidak melar).

## Yang BUKAN penyebabnya

Bukan resolusi laptop itu, bukan penskalaan Windows 125/150%, bukan
WebView2/Tauri, bukan build yang berbeda. Mesin mana pun rusak sama persis
begitu pesan tahapnya sepanjang itu — sudah dibuktikan dengan menyuntikkan
kalimat yang sama ke halaman yang berjalan di sini.

## Kenapa lolos dari semua pengukuran kita

Dua celah, dan keduanya lebih penting daripada bug-nya sendiri:

1. **`scripts/measure-ui.mjs` hanya mengukur tinggi** (`scrollHeight` vs
   `clientHeight`). Aturan "jendela tidak boleh bergulir" di `CLAUDE.md` pun
   selama ini cuma dibicarakan sebagai gulir tegak. Sumbu mendatar tidak pernah
   diukur sekali pun.
2. **Halaman selalu diukur dalam keadaan diam**, tanpa job berjalan. Teks yang
   panjangnya ditentukan engine — pesan tahap, path, nama model, galat — tidak
   pernah ikut masuk ke pengukuran, padahal justru itu masukan yang bisa
   merusak tampilan.

Kalau pengukurannya ditambah nanti: ukur `scrollWidth` juga, dan ukur dengan
teks tahap terpanjang yang bisa dihasilkan engine, bukan dengan panel kosong.

## Suntikan yang menambalnya

`gui/app/globals.css:322`

Dari:

```css
.run-progress { flex: 1 0 100%; }
```

Jadi:

```css
.run-progress { flex: 1 0 100%; min-width: 0; }
```

`flex-basis: 100%` sudah menetapkan lebar yang benar; yang melanggarnya adalah
ukuran minimum otomatis flex item. `min-width: 0` mencabut lantai itu, sehingga
`.stage` kembali terpotong elipsis seperti maksud komentar di atasnya.

Verifikasi: `./build.sh` (CSS ikut `gui/out`, wajib build ulang) lalu
`node scripts/measure-ui.mjs` — `.screen-col over=0`, dan panel Start
menampilkan `…at http://localho…`.
