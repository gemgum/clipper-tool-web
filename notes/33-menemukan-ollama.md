# Menemukan Ollama sendiri — 6 Agustus 2026

Dipicu satu laporan yang tidak bisa dijelaskan: GUI menampilkan
**"llama3.1:latest — ready"**, tapi job berhenti dengan
`transcript correction failed: Ollama (qwen2.5) …`.

## Apa yang diperiksa (mesin pelapor: Windows + WSL)

| Diperiksa | Hasil |
| --- | --- |
| `ollama list` di PowerShell | perintahnya tidak ada |
| `Get-Process ollama*` di Windows | kosong |
| `%LOCALAPPDATA%\Programs\Ollama` | tidak ada |
| `http://127.0.0.1:11434/api/tags` **dari Windows** | **menjawab** |
| `http://127.0.0.1:11434/api/tags` dari WSL | menjawab, isi identik (digest & `modified_at` sama) |

Kesimpulannya: **tidak ada Ollama di Windows.** Yang menjawab di localhost
Windows adalah Ollama di WSL, lewat penerusan port WSL2. Aplikasi Windows-nya
bekerja karena kebetulan susunan itu meneruskan port — dan akan berhenti bekerja
begitu WSL dimatikan, tanpa satu kalimat pun yang menjelaskan kenapa.

## Kenapa nama model bisa berbeda dari yang terlihat

Nilai kosong berubah jadi bawaan **di dua tempat yang tidak saling tahu**:

- `config.Validate` → `OllamaModel = "qwen2.5"` bila kosong;
- `ollama.New` → `model = "qwen2.5"` bila kosong.

Dan nama tersimpan bisa **tanpa tag** (`"qwen2.5"`) sementara yang terpasang
bernama `"qwen2.5:latest"` — cukup untuk membuat kotak pilihan menampilkan satu
hal sementara yang terkirim hal lain. Tidak ada satu pun tempat yang menyatakan
model apa yang BENAR-BENAR dipakai sebelum pekerjaan dimulai; namanya hanya
muncul kalau job gagal.

## Yang dikerjakan

1. **`score/ollama/discover.go`** — alamat Ollama DICARI, tidak dipaku:
   `OLLAMA_HOST` → `localhost` → `127.0.0.1` → gerbang WSL (`ip route`) →
   nameserver `resolv.conf` → `host.docker.internal`. Semua diketuk serentak,
   yang **paling awal di daftar** menang (bukan yang tercepat menjawab — dua
   alamat bisa menunjuk Ollama berbeda dengan model berbeda). Hasil di-cache 30
   detik.
   - Alamat **publik ditolak** sebagai kandidat: di sebagian WSL `resolv.conf`
     berisi DNS publik (1.1.1.1), dan mengetuk port di alamat internet milik
     orang lain bukan hal yang boleh dilakukan aplikasi ini diam-diam.
2. **`config.DefaultOptions().OllamaURL` jadi kosong**, dan `Validate` tidak
   lagi mengisinya — kosong berarti "temukan sendiri". Alamat yang dipaku benar
   hanya untuk satu dari tiga susunan.
3. **`pipeline/ollama_resolve.go`** — satu tempat yang menentukan Ollama MANA dan
   model MANA. Model dicocokkan dengan yang benar-benar terpasang (tanpa/dengan
   tag); yang diminta tapi tidak ada **tidak diganti diam-diam** — job berhenti
   dan pesannya menyebut apa yang terpasang. Nama lengkap + alamat ikut ke log
   sebelum job jalan: `Ollama (llama3.1:latest) at http://… — this computer`.
4. **Bawaan model jadi `llama3.1`**, bukan qwen2.5 — `notes/22` sudah
   menyimpulkan itu; kodenya yang belum ikut.
5. **Tanpa Ollama, jangan memaksa Ollama.** Saat status pertama datang dan tidak
   ada Ollama di mana pun, GUI menyetel mesin skor ke **heuristik** dan mematikan
   koreksi transkrip, dengan lambang peringatan yang menjelaskan sebabnya.
   - Ini BUKAN pelanggaran `notes/12`: yang dipilih adalah **setelan awal yang
     terlihat di layar**, bukan penggantian mesin diam-diam saat job berjalan.
     Begitu Start ditekan, mesin yang tertulis itulah yang dipakai — dan kalau
     gagal, job tetap berhenti.
6. **Alamat yang ditemukan ditampilkan**: `/api/ollama/status` mengembalikan
   `url` + `where`, dan GUI memasangnya sebagai tooltip label mesin/model.
   "Ollama jalan" tanpa menyebut DI MANA membuat susunan Windows+WSL mustahil
   didiagnosis — persis kasus di atas.

## Yang TIDAK terbukti

Sebab pasti kenapa job itu memakai qwen2.5 padahal layar menunjukkan llama3.1
**belum terbukti** — tidak ada log dari mesin itu yang menyimpan pilihan saat
job dibuat. Yang dikerjakan di atas menutup setiap jalur yang bisa
menghasilkannya, dan mulai sekarang nama model + alamatnya tercatat sebelum
pekerjaan dimulai, jadi kalau terulang ia bisa dibaca, bukan ditebak.

## Tambahan sore itu juga

- **Nama baris di Requirements = nama berkas yang benar-benar dipakai.**
  "whisper.cpp" adalah nama proyek; yang dijalankan engine bernama
  `whisper-cli.exe`, dan itu yang harus dicari orang saat memeriksanya sendiri
  atau mengizinkannya di antivirus. Sekarang namanya diturunkan dari path yang
  ditemukan (`whisper-cli`, `ggml-base.bin`, `chrome.exe`), termasuk untuk model
  yang belum diunduh — satu daftar tidak boleh memakai dua gaya penamaan.
- **Baris Ollama menyebut SISTEMNYA**: `Ollama — WSL (this Linux system)` dengan
  alamatnya di tempat path. `Where()` kini menyimpulkan OS-nya, bukan cuma
  alamat: di Windows, `localhost` yang menjawab TANPA `ollama.exe` terpasang
  berarti Ollama WSL yang portnya diteruskan (dan ikut mati saat WSL dimatikan).
- **Lambang peringatan di sel tanpa label MELAYANG.** Di `.field-check` (sel yang
  isinya cuma tombol) ia dulu berdiri sebagai baris sendiri dan mendorong
  tombolnya turun. Sekarang `position: absolute` di pojok kanan-atas sel, dengan
  ruang permanen 8 px dari `padding-top` kisinya.

  Terukur, bukan diperkirakan: tepi atas tombol "Analyse" dan "Paragraphs"
  **91 px sebelum dan 91 px sesudah** lambang muncul; tinggi panelnya **420 px**
  di kedua keadaan.

## Dua daftar model jadi satu (`gui/app/ollama.ts`)

Tab klip menampilkan `llama3.1:latest · 8.0B · 4.9 GB · ✓ ready`, tab kartu cuma
`llama3.1:latest`. Data yang sama persis, dua tampilan — dan yang kedua tidak
pernah ikut diperbaiki karena kodenya ditulis dua kali.

Sekarang keduanya memakai satu hook: `useOllama(active)` (status + daftar +
penyegaran berkala/saat fokus) dan `modelOptions()` (opsi + saran yang belum
terpasang). Terukur, keduanya kini mengeluarkan baris yang sama persis:

```
llama3.1:latest | 8.0B · 4.9 GB · ✓ ready
qwen2.5:latest  | 7.6B · 4.7 GB · ✓ ready
gemma2          | needs download
```

Tab kartu ikut mendapat yang selama ini hanya ada di tab klip: penyamaan nama
bertag, pemilihan otomatis model yang siap, dan lambang peringatan saat Ollama
tidak terjangkau.

### Sistem asal ditempel di labelnya

`StatusInfo.OS` — satu kata (`Windows` · `WSL` · `Linux` · `macOS` · `remote`) —
ditempel di label: **"Model · WSL"**. Kalimat panjangnya (`Where`) tinggal di
tooltip.

Satu jebakan yang langsung kena saat dicoba: label "Ollama model · WSL" MELIPAT
jadi dua baris di kolom selebar ~110 px, dan itu menaikkan tinggi seluruh baris
kendalinya — menambah satu kata pun bisa menggeser UI. Karena itu labelnya
dipendekkan jadi "Model" DAN label di kisi sekarang `nowrap` + elipsis. Terukur
sesudahnya: keempat kendali baris itu **top 91, tinggi 32**.

## Sistem asal di TIAP baris daftar, bukan cuma di label

Label "Model · WSL" ternyata belum cukup: yang dilihat orang saat memilih adalah
daftarnya, dan daftar itu datang dari SATU Ollama tertentu. Pada mesin yang punya
Windows dan WSL sekaligus, "daftar ini dari mana" adalah hal pertama yang perlu
dipastikan. Sekarang tiap barisnya:

```
llama3.1:latest   8.0B · 4.9 GB · ✓ ready · WSL
qwen2.5:latest    7.6B · 4.7 GB · ✓ ready · WSL
gemma2            needs download
```

Baris saran (belum terpasang) sengaja tanpa penanda sistem — ia belum berasal
dari mana pun.

## Ukuran jendela minimum dikunci di 1240×860

Diminta pemilik proyek 6 Agustus 2026, dan angkanya bukan selera — ini hasil
ukur seluruh halaman di lima ukuran (`SIZES=… node scripts/measure-ui.mjs`):

| Ukuran | klip (main/col) | kartu | riwayat |
| --- | --- | --- | --- |
| **1240×860** | **0 / 0** | **0 / 0** | 0 |
| 1240×800 | 23 / 0 | 47 / 0 | 0 |
| 1240×760 | 63 / 38 | 87 / 0 | 0 |
| 1100×860 | 566 / 105 | 186 / 0 | 0 |
| 1024×860 | 514 / 105 | 105 / 0 | 0 |

1240×860 satu-satunya yang tidak menggulir apa pun; menyempitkan lebar ke 1100
langsung membuat kolom kiri melimpah 566 px (kolom kanan lebarnya tetap, jadi
seluruh penyempitan ditanggung pratinjau). Karena itu `minWidth`/`minHeight` di
`desktop/src-tauri/tauri.conf.json` disamakan dengan ukuran bawaannya —
jendelanya tidak bisa lagi diperkecil, hanya diperbesar.

**Konsekuensi yang harus diketahui:** layar 1366×768 tidak muat — 860 + bilah
judul + taskbar ≈ 940 px. Di layar seperti itu jendelanya akan terpotong bagian
bawahnya. Kalau suatu saat itu jadi masalah, yang harus dikerjakan adalah
membuat halaman klip muat di jendela lebih pendek (mis. kelompok setelan yang
bisa diciutkan), BUKAN menurunkan angka minimumnya kembali — tabel di atas
menunjukkan apa yang terjadi kalau diturunkan.
