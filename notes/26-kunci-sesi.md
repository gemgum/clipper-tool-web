# Port acak & kunci sesi

Selesai 4 Agustus 2026. Penghalang nomor 2 di `23-aplikasi-desktop.md`.

> **Sudah dilewati sebagian, 5 Agustus 2026.** Kunci tidak lagi dititipkan di
> query maupun `sessionStorage`: ia tinggal di cookie `HttpOnly; SameSite=Strict`
> yang dipasang engine saat halaman pertama dibuka, dan `/api/` **menolak**
> `?token=`. Alasan & akibatnya di `30-pengerasan-dikerjakan.md` nomor 3.
> Bagian "port acak", "engine.json 0600", dan "asal permintaan" di bawah tetap
> berlaku apa adanya.

## Soalnya

Engine mendengar di `127.0.0.1:8787` dengan CORS `*` dan tanpa kunci. Di mesin
pengembang itu tidak apa-apa. Di mesin pengguna artinya:

- **setiap program** di komputer itu bisa memerintah engine — porta selalu sama,
  jadi tidak perlu mencari;
- **setiap halaman web** yang sedang dibuka pengguna bisa melakukan hal yang
  sama lewat browsernya, sebab CORS `*` mengizinkan siapa pun membaca jawabannya.

Sejak ada `/api/browse` (lihat `24-berkas-lokal.md`), yang bisa dilakukan bukan
cuma memerintah engine, tapi juga **menelusuri berkas pengguna**. Itu yang
membuat bagian ini berhenti bisa ditunda.

## Yang sekarang berlaku

**Port.** Tanpa `-addr`: 8787 dari checkout sumber (GUI pengembangan menunjuk ke
sana), **port acak** saat terpasang.

**Kunci sesi.** Dibuat baru setiap engine dijalankan, wajib di setiap permintaan
kecuali `/api/health`. Diterima lewat header `X-Clipper-Token`,
`Authorization: Bearer`, atau query `?token=`. Query wajib ada: `EventSource`,
`<video src>`, dan tautan unduh dibentuk browser dan tidak bisa membawa header.
Perbandingannya waktu-tetap.

**Asal permintaan.** CORS tidak lagi `*`. Hanya halaman dari mesin ini sendiri
(`localhost`, `127.0.0.1`, `::1`) dan skema shell desktop (`tauri://`, `app://`,
`file://`) yang diizinkan; selain itu dijawab 403 sebelum sampai ke handler.
Origin dipantulkan, bukan `*` — dengan `*` browser menolak mengirim header kunci
pada permintaan yang butuh preflight.

## Cara shell nanti menemukan engine

Berkas **`<DataDir>/engine.json`**, izin 0600:

```json
{ "url": "http://127.0.0.1:38887", "port": 38887,
  "token": "…", "pid": 31708, "version": "0.1.0-mvp" }
```

Berkas, bukan argumen baris perintah: port baru diketahui SETELAH engine
berhasil mendengar, dan kunci di baris perintah terlihat oleh setiap program
lain lewat daftar proses. 0600 karena kunci itu setara kata sandi ke seluruh isi
berkas pengguna.

Shell membacanya, lalu membuka jendela GUI dengan `?token=…`. GUI menyimpannya
di `sessionStorage` dan membersihkannya dari bilah alamat (`gui/app/engine.ts`).
sessionStorage, bukan localStorage: kuncinya berganti tiap engine dijalankan,
jadi menyimpannya lebih lama hanya membuat kunci basi terkirim.

Bagian ini yang tadinya disebut "menunggu keputusan shell". Ternyata tidak:
apa pun yang menang — Chrome `--app=` atau Tauri — keduanya hanya perlu membaca
satu berkas JSON dan membuka satu URL.

## Yang sengaja dibiarkan longgar

Dari checkout sumber, kunci **mati** secara bawaan: GUI `npm run dev` di :3000
tidak punya cara menerima kunci saat halaman pertama dibuka. Nyalakan dengan
`clipper serve -token on`, lalu buka `http://localhost:3000/?token=<kunci>` —
begitulah alur ini diuji.

Banner `serve` selalu menyebut keadaannya, sebab tidak ada flag yang
memilihnya di pemakaian normal:

```
  key     : on (a new key for every run, written to the address file)
  address : /home/…/data/engine.json
```
