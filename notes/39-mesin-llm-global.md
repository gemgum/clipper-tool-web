# Mesin LLM jadi satu, global — Rencana & Keputusan

Tanggal diskusi: 18 Agustus 2026. Sisi engine **sudah dikerjakan** (lihat bagian
paling bawah); sisi GUI belum.

Masalahnya: tiap tab memilih mesin LLM dengan caranya sendiri. Halaman klip
menyebutnya "Scoring engine" dengan pilihan mode+mesin, tab kartu menyebutnya
"Engine" dengan pilihan lain, tab pembuat berita punya yang ketiga. Kunci Claude
hanya bisa diisi dari tab klip, padahal ia berlaku se-aplikasi. Menambah satu
penyedia berarti menyentuh tiga tempat.

## Keputusan pemilik proyek

1. **Kunci API pindah ke setelan utama.** Satu tempat, bukan di panel tiap tab.
2. **URL, nama model, dan sejenisnya tetap bisa diisi manual.** Sebabnya jelas:
   penyedia menambah dan mengganti model terus-menerus, dan daftar yang dipaku
   di kode akan basi lebih cepat daripada aplikasinya dirilis ulang.
3. **Mesin yang kuncinya BELUM diisi tidak muncul di dropdown.** Pengguna harus
   membuka Setelan untuk mesin yang ia pakai.
4. **Berlaku sama untuk ketiga tab**: klip, kartu berita, pembuat berita.
   Satu komponen yang sama, bukan tiga yang mirip — supaya pengembangannya
   stabil.
5. **Blok SERVER dibuang dari popup setelan.** Alamat server pindah ke setelan
   utama bersama yang lain.

## Yang PANTAS diisi manual, dan yang tidak

Pertanyaan pemilik proyek: apa lagi selain kunci?

| Isian | Manual? | Alasan |
| --- | --- | --- |
| Kunci API | **ya** | rahasia, hanya pengguna yang punya |
| Base URL | **ya** | penyedia memindahkan endpoint; dan inilah yang membuat gateway (LiteLLM, OpenRouter, Azure) bisa dipakai tanpa kode baru |
| Nama model | **ya** | model baru terbit tiap beberapa minggu — daftar yang dipaku pasti basi |
| Nama tampilan | ya | supaya "OpenRouter (Gemini)" terbaca sebagai apa adanya |
| Temperature | **tidak** | ditentukan TUGASNYA, bukan penyedianya: ekstraksi fakta jalan di 0.1, pemilihan momen di 0.4. Dijadikan satu isian global, salah satunya pasti rusak |
| Max tokens | **tidak** | baru saja jadi bug nyata: 4096 memotong JSON di tengah dan job gagal total (notes/38). Sudah dipatok 8192 di engine. Isian yang salah isi menghidupkan bug itu lagi |
| Nama header auth | **tidak** | semua endpoint OpenAI-compatible memakai `Authorization: Bearer`. Isian ini cuma menambah cara untuk salah |
| Timeout | **tidak** | sudah 12 menit, dan angkanya lahir dari kejadian nyata (memuat model 8B di mesin tanpa GPU) |

**Prinsipnya:** yang manual adalah yang hanya pengguna yang tahu. Yang bisa
ditanyakan ke servernya, JANGAN diketik — tanyakan.

## Tombol Test bukan cuma "tersambung"

Sekarang tombol Test menjawab hidup/mati. Ia bisa mengerjakan lebih banyak, dan
tiap tambahannya menghapus satu isian manual:

| Yang ditanyakan | Menggantikan |
| --- | --- |
| `GET {base}/v1/models` | pengguna mengetik nama model dari ingatan |
| satu panggilan kecil berskema | tebakan apakah penyedia menghormati `response_format: json_schema` |

Yang kedua penting dan sering diremehkan. **Seluruh pagar fakta pembuat berita
bergantung pada balasan berskema.** Ollama menjaminnya di sisi server, Claude
tidak menerima skema sama sekali (bentuknya diminta lewat prompt — sudah
ditangani), dan penyedia OpenAI-compatible berbeda-beda ketaatannya. Kalau ada
yang membalas JSON asal-asalan, gejalanya BUKAN galat melainkan job yang gagal
parsing di tengah jalan.

Jadi hasil Test disimpan sebagai sifat mesin itu, bukan sekadar lampu hijau
sesaat. Mesin yang gagal uji skema tetap boleh dipakai — tapi ditandai, dan
tab pembuat berita berhak memperingatkan sebelum dipakai di sana.

## Bentuk baku pemilih mesin

Satu komponen, dipakai apa adanya di ketiga tab:

```
Engine  [ dropdown ]     Model · <sumber>  [ dropdown + bisa diketik ]   [ Test ]
```

- `<sumber>` = keterangan dari mana daftar modelnya datang ("Ollama", "WSL",
  "Gemini") — sudah dipakai tab kartu hari ini, dan memang berguna.
- Dropdown model **boleh diketik**: daftar dari server adalah bantuan, bukan
  pagar. Model yang baru terbit kemarin harus bisa dipakai hari ini.
- Isi dropdown Engine = mesin lokal yang terdeteksi + mesin cloud yang
  **kuncinya sudah ada**.

### Satu ganjalan pada keputusan "tidak muncul kalau belum disetel"

Aturannya jelas dan saya ikuti. Tapi akibatnya: pengguna yang belum pernah
mengisi kunci Gemini **tidak akan pernah tahu Gemini didukung** — dropdown-nya
cuma berisi Ollama, dan tidak ada satu pun tanda bahwa ada pilihan lain.

Usul kecil yang menutup itu tanpa melanggar keputusannya: baris terakhir
dropdown yang selalu ada dan bukan mesin —

```
Ollama
Claude
──────────────
Add an engine…      → membuka Setelan
```

Bukan mesin kelabu yang bisa terpilih, cuma pintu. **Perlu keputusan.**

## Setelan utama: apa isinya sesudah ini

Halaman Requirements jadi tempat SEMUA yang disetel sekali lalu dilupakan:

| Bagian | Isi | Status |
| --- | --- | --- |
| **AI engines** | satu baris per mesin: nama, base URL, kunci, model bawaan, tombol Test + hasilnya | **baru** |
| Components | whisper.cpp, ffmpeg, model, Chrome | sudah ada |
| Separate applications | aplikasi LLM lokal (Ollama, LM Studio, Jan, llama.cpp, KoboldCpp, GPT4All) | sudah ada; **alamat server pindah ke sini** |
| Folders | folder keluaran | sudah ada |

Baris mesin cloud dan blok "Separate applications" sengaja **tidak digabung**:
yang pertama tentang kredensial, yang kedua tentang program yang harus dipasang
sendiri di komputer ini. Menyatukannya membuat "belum ada kuncinya" dan "belum
terpasang programnya" terlihat seperti masalah yang sama, padahal
penyelesaiannya sama sekali berbeda.

## Popup setelan: apa yang tersisa

Blok **SERVER** dibuang seluruhnya (alamat + kunci server + tombol Save). Yang
tersisa:

```
Settings                    0.8.1
LANGUAGE   [English] [Indonesia]
COMPONENTS  ● Library    ready
            ● Transcribe ready
            ● LLM        ready
            ● Chrome     ready
Open the full Requirements page →
```

Popup jadi apa yang seharusnya: **pemberi kabar, bukan tempat menyetel.** Satu
saklar yang benar-benar sekali pakai (bahasa), sisanya lampu status dan pintu ke
halaman penuh.

## Diputuskan 18 Agustus 2026 (lanjutan)

1. **Baris "Add an engine…" DIPAKAI**, dan mengarahkan pengguna ke setelan utama
   untuk mengisi kuncinya di sana.
2. **Keempat penyedia dikerjakan sekaligus**: Claude, ChatGPT (OpenAI), Gemini,
   DeepSeek. Kuncinya diisi sendiri oleh pemilik proyek.
3. **Kunci tetap di `.env`.** Konsisten dengan yang sudah ada, dan CLI ikut
   kebagian tanpa kerja tambahan. Risiko yang diterima sadar: berkas teks polos
   berisi empat kunci berbayar (notes/28).
4. **"Mode: Offline/Hybrid" diperiksa ulang; kalau tidak punya pekerjaan lagi,
   dibuang.**

## Utang penamaan yang diterima sadar

Klien OpenAI-compatible tinggal di paket `score/ollama`, dan sesudah ini ia juga
melayani ChatGPT, Gemini, dan DeepSeek — yang sama sekali bukan Ollama.
Namanya memang sudah historis sejak notes/35 ("Namanya menyebut Ollama karena
begitulah ia lahir"). Memindahkannya berarti menyentuh discover.go, pipeline,
dan api sekaligus untuk nol perubahan perilaku; dibiarkan, dan dicatat di sini
supaya sesi berikutnya tidak mengira itu kelalaian.

## Dua ganjalan teknis yang ditemukan saat memeriksa kodenya

**1. `/v1` dipaku di klien.** `completeOpenAI` menyusun `{base}/v1/chat/completions`
dan `openAIModels` menyusun `{base}/v1/models`. Itu benar untuk OpenAI dan
DeepSeek, tetapi endpoint OpenAI-compatible milik Gemini adalah
`…/v1beta/openai/chat/completions` — tidak ada `/v1` di tengahnya. Jadi klien
butuh satu field `Path` (bawaan `/v1`) supaya Gemini bisa memakai
`/v1beta/openai`. Perubahannya aditif; nilai kosong berarti perilaku lama.

**2. Kunci dibaca dari satu variabel global.** `apiKey()` membaca `LLM_API_KEY`
untuk SEMUA server OpenAI-compatible. Dengan empat penyedia cloud, tiap klien
harus membawa kuncinya sendiri (`Client.APIKey`), dan `LLM_API_KEY` tinggal jadi
cadangan untuk server lokal di belakang gateway.

## Kenapa ini dikerjakan sekarang

Bukan kerapian. Pembuat berita menambahkan pemilih mesin KETIGA yang mirip tapi
tidak sama, dan fitur berikutnya akan menambah keempat. Menyatukannya sekarang
berarti menambah penyedia baru = satu baris di satu tabel, bukan menyentuh empat
berkas dan berharap tidak ada yang terlewat.


## Sisi engine: SUDAH DIKERJAKAN (18 Agustus 2026)

`engine/internal/api/engines.go` — satu tabel `engineDefs`, lima mesin:
`ollama` (lokal), `claude`, `openai`, `gemini`, `deepseek`.

| Endpoint | Isi |
| --- | --- |
| `GET /api/engines` | seluruh mesin + alamat/model yang BERLAKU + `has_key` + `ready` |
| `POST /api/engines` | simpan kunci/alamat/model satu mesin ke `.env` |
| `POST /api/engines/test` | uji: sambungan, daftar model, DAN ketaatan skema |

`EngineFor(id, model)` menggantikan `LLMEngine(provider, apiKey, claudeModel,
ollamaModel, ollamaURL)`. Dipakai bersama oleh tab kartu, pembuat berita, dan
CLI (`clipper write -engine … -model …`).

**`.env` jadi satu-satunya sumber kebenaran.** Penyimpanan kunci sekarang
`os.Setenv` DAN menulis berkas — sebelumnya kunci Claude hanya masuk manager dan
berkas, jadi ia belum terbaca sampai aplikasi dijalankan ulang. Efek sampingnya
bagus: CLI ikut kebagian tanpa kerja tambahan.

### Yang terbukti dari uji langsung

| Uji | Hasil |
| --- | --- |
| `test` mesin lokal | `ok:true`, `schema:true`, balasan `{"ok":true}`, daftar model terbaca |
| `test` tanpa kunci | *"Gemini has no API key yet — add it on the Requirements page"* |
| `test` DeepSeek, kunci salah | benar-benar sampai ke `api.deepseek.com` dan mengembalikan pesan penyedianya sendiri |

Yang ketiga penting: jalur OpenAI-compatible ke penyedia cloud memang jalan,
bukan cuma dianggap jalan. Pesan galatnya sempat tercetak sebagai sintaks map Go
(`map[code:… message:…]`) dan sudah dibetulkan — yang membaca pesan itu pengguna,
bukan pemrogram.

### Perubahan penopang di paket lain

- `score/ollama`: `Client.Path` (bawaan `/v1`) dan `Client.APIKey`. Keduanya
  aditif — nilai kosong berarti perilaku lama persis.
- `score/llm`: `Client.BaseURL`, supaya Claude pun bisa diarahkan ke gateway.
- `ollama.Models(ctx, url, path, key)` diekspor untuk mengisi pilihan model.

### Nama field lama masih diterima

`/api/news/analyze` dan `/api/posts` menerima `engine`+`model` (baku) DAN
`provider`/`ollama_model`/`llm_model` (lama). Sengaja: sisi GUI belum diperbarui,
dan menukar keduanya sekaligus berarti ada jendela waktu tab yang rusak.
**Dibuang setelah GUI-nya ikut baku.**

## Sisi GUI: SUDAH DIKERJAKAN (18 Agustus 2026)

| Yang dikerjakan | Berkas |
| --- | --- |
| Komponen pemilih mesin bersama | `gui/app/engine-picker.tsx` |
| Bagian "AI engines" di setelan | `gui/app/requirements/engines.tsx` |
| Blok SERVER dibuang dari popup | `gui/app/settings-menu.tsx` |
| Tab kartu & pembuat berita memakainya | `news/page.tsx`, `writer/page.tsx` |

Bentuknya seperti disepakati: `Engine · Model · <sumber> · Test`, dengan baris
terakhir dropdown `Add an engine…` yang membuka halaman setelan. Kotak model
**boleh diketik** dan daftarnya terisi sendiri dari `/api/engines/{id}/models` —
satu GET, sengaja dipisah dari `/test` yang memanggil LLM sungguhan, sebab
mengganti pilihan mesin tidak boleh berbiaya token.

Diukur ulang (`scripts/measure-ui.mjs`): klip, kartu, dan pembuat berita tetap
`0/0` di 1240×860.

### Dua hal yang lahir dari MELIHAT hasilnya

**1. Kotak model kosong itu jebakan.** Sesudah pemilih lama diganti, kotak model
mesin lokal kosong — dan kosong berarti engine memakai bawaan kliennya
(`qwen2.5`). Di komputer yang cuma punya llama3.1, job itu gagal saat
dijalankan. Pemilih sekarang mengisi sendiri dengan model pertama yang
DIAKUI server, mengembalikan perilaku lama tab kartu.

**2. Lima mesin × dua baris isian = ~875 px**, dan daftar komponen — gunanya
halaman itu sejak awal — terdorong jauh ke bawah lipatan. Isiannya dijadikan
satu baris (kunci · alamat · model · tombol); tingginya turun jadi ~560 px.

## "Mode: Offline/Hybrid" — diperiksa, dan JANGAN dibuang begitu saja

Diperiksa seperti diminta. Ternyata ia masih punya SATU pekerjaan yang tidak
dikerjakan siapa pun:

| Tempat | Yang dilakukan Mode |
| --- | --- |
| `config.go:374` | mengisi Provider bila kosong (offline→ollama, hybrid→claude) — **sudah tergantikan** pemilih mesin |
| `main.go:196` | memperingatkan "hybrid tapi kunci kosong" — **sudah tergantikan** status mesin |
| `pipeline.go:351` | memilih mesin **KOREKSI TRANSKRIP**: hybrid → Claude, selain itu → Ollama |

Yang ketiga itu pekerjaan sungguhan, dan tidak kelihatan dari namanya. Artinya
halaman klip diam-diam punya DUA pilihan mesin: satu bernama "Scoring engine",
satu lagi menyamar sebagai "Mode".

**Usul: buang "Mode", ganti dengan pemilih mesin KEDUA** berlabel jujur —
"Koreksi transkrip". Itu sekaligus membuka hal yang selama ini mustahil:
menilai momen dengan Claude sementara koreksi transkrip jalan di model lokal
yang gratis, atau sebaliknya. **Perlu keputusan.**

## Sisa pekerjaan

1. **Halaman klip ikut memakai pemilih bersama.** Ini yang paling berat dan
   sengaja ditinggal terakhir: `pipeline` merakit kliennya SENDIRI
   (`switch p.Opts.Provider` di dua tempat) dan tidak bisa mengimpor `api`
   (`api` sudah mengimpor `pipeline`). Jadi tabel mesin harus pindah dari
   `internal/api` ke paket sendiri, mis. `internal/engines`, supaya keduanya
   memakainya. Sampai itu terjadi, halaman klip tetap memakai panelnya sendiri
   dan HANYA mengenal claude/ollama/heuristik.
2. Keputusan soal "Mode" di atas.
3. Buang penerimaan nama field lama (`provider`, `ollama_model`, `llm_model`)
   di `/api/news/analyze` dan `/api/posts` setelah 1 selesai.


## DeepSeek: empat kegagalan berturut-turut, dan apa yang diajarkannya

Diuji 18 Agustus 2026 dengan kunci sungguhan. Tiap kegagalan menyingkap satu
dugaan yang selama ini benar HANYA karena semua uji sebelumnya memakai Ollama.

**1. Alamatnya diisi endpoint gaya Anthropic.** DeepSeek menyediakan
`https://api.deepseek.com/anthropic` untuk Claude Code, dan itu yang tersalin ke
setelan. Klien kita memanggilnya sebagai OpenAI-compatible → 404 berbadan
kosong. Alamat yang benar `https://api.deepseek.com`.

Perbaikan sisi kode: pesan galat kini menyebut **alamat LENGKAP** yang dipanggil,
bukan base-nya saja. Dengan jalur penuh, kekeliruan ini terbaca dalam sedetik.

**2. DeepSeek MENOLAK `response_format: json_schema`.** Komentar lama di
`openai.go` menduga "server yang tidak sanggup akan mengabaikan field ini".
Ternyata tidak: seluruh permintaan ditolak. Sekarang klien mencoba ulang tanpa
skema dan mengingatnya per alamat.

**3. Tanpa skema, model mengarang bentuk balasannya sendiri** — larik telanjang
berisi `{"paragraph":…,"fact":…}`, bukan `{"facts":[{"text":…}]}`. Sebabnya
memalukan: **prompt-nya tidak pernah menyebutkan bentuk JSON yang diminta** —
selama ini skema server yang menanggungnya, dan Ollama selalu memaksakannya.
Bentuknya kini tertulis di kedua prompt, dan `extractJSON` mengenali larik.

**4. Model DeepSeek BERNALAR** (`reasoning_content`, `reasoning_tokens`), dan
penalaran memakai jatah `max_tokens` yang sama dengan jawabannya. Pada artikel
1.500 kata, 8192 token habis sebelum satu huruf jawaban keluar — yang sampai ke
pemanggil cuma balasan kosong, atau JSON terpotong di tengah.

Sekarang: `finish_reason: "length"` dilaporkan sebagai galat yang menyebut
sebabnya, dan jatah dilipatkan (4×, batas 32768) HANYA untuk model yang terbukti
membutuhkannya — diingat per alamat+model. Tidak dinaikkan untuk semua, sebab
pada model lokal `num_predict` besar berarti model bertele-tele menghabiskan
menit-menit (llama3.1 memang bertele-tele, lihat pagar pengulangan notes/38).

**Dan satu bug dari perbaikannya sendiri:** kedua kelonggaran ditulis sebagai
dua penambalan berurutan, jadi jalur "tolak skema" mengembalikan hasilnya
langsung dan penanganan jatah tidak pernah terpakai. Diganti satu putaran
berbatas tiga percobaan, di mana kedua kelonggaran bisa berlaku bersama.

**Pelajaran yang lebih besar:** "jalan di Ollama" bukan "jalan". Ollama
memaksakan skema di sisi server, dan itu menyembunyikan tiga dugaan sekaligus —
bentuk balasan tidak perlu ditulis di prompt, server yang tidak sanggup akan
mengabaikan field, dan jatah token cuma soal panjang jawaban. Penyedia berikutnya
yang ditambahkan harus diuji ujung-ke-ujung, bukan cuma dengan tombol Test.

**Test kini membedakan dua hal yang dulu satu:** `schema` (balasannya BERBENTUK
benar) dan `strict` (servernya MEMAKSAKAN bentuk itu). DeepSeek menjawab
`schema:true, strict:false` — dan hanya yang kedua yang menentukan apakah
balasan panjang bisa dipercaya.


## Kegagalan KELIMA, dan yang paling halus: model menerjemahkan

Sesudah keempat perbaikan di atas, job DeepSeek tiga artikel berhasil —
**505 kata**, di dalam target 400–700, dengan 48 klaim terpetakan. Bandingkan
llama3.1 pada bahan yang sama: 84–267 kata, peta klaim sering kosong.

Tetapi angka fakta yang DITOLAK melonjak: 9, 6, dan 6 dari 29, 23, dan 18.
Isinya menjelaskan sebabnya:

```
- "The term Paskibra is generally used for groups tasked with raising the flag…"
  alasan: no paragraph in the article shares enough wording with this
```

DeepSeek **menerjemahkan** artikel Indonesia jadi fakta berbahasa Inggris. Pagar
pembumian bekerja dengan irisan kata terhadap paragraf aslinya, jadi fakta
terjemahan tidak berbagi satu kata pun — dan dibuang sebagai tak bersumber.
Sepertiga pekerjaan hilang tanpa satu pun dari keduanya salah.

Prompt tahap 1 kini melarang menerjemahkan secara eksplisit, dan menyebutkan
akibatnya supaya alasannya jelas bagi modelnya sendiri.

Ini juga menjelaskan pelanggaran `coverage` di job yang sama: fakta khas salah
satu sumber ikut terbuang bersama terjemahannya.

## Harga yang harus disadari: DeepSeek jauh lebih lambat

```
Read Antara News    2m50s    29 fakta, 9 ditolak
Read Antara News    2m15s    23 fakta, 6 ditolak
Read Antara News    2m18s    18 fakta, 6 ditolak
Write article      10m13s    505 kata, diperbaiki sekali
TOTAL              17m36s
```

llama3.1 pada bahan yang sama: **3m34s**. DeepSeek lima kali lebih lambat, sebab
ia bernalar dan jatahnya kini 32.768 token. Mutu artikelnya jelas lebih baik —
tapi ini bukan pertukaran yang bisa diabaikan, dan pilihan mesin per tab memang
ada untuk itu: baca fakta di model cepat, tulis di model pandai.

**Belum diuji:** apakah `deepseek-v4-flash` lebih cepat secara berarti. Keduanya
bernalar, jadi belum tentu.


## Mesin dipisah per tahap (18 Agustus 2026)

Angka dari uji DeepSeek yang membuatnya jadi keharusan, bukan kelengkapan:

| | Tahap 1 (baca fakta) | Tahap 2 (menulis) |
| --- | --- | --- |
| Berapa kali dipanggil | **sekali per artikel** (sampai 5) | **sekali** |
| Sifat pekerjaannya | menyalin-ulang | menulis |
| llama3.1 | cukup — 18–29 fakta per artikel | 84–267 kata, di bawah target |
| DeepSeek | 2m15s–2m50s **per artikel** | 505 kata, di dalam target |

Memaksa keduanya memakai mesin yang sama berarti memilih antara membayar mahal
LIMA KALI untuk pekerjaan menyalin-ulang, atau menerima artikel buruk demi
hemat. Sekarang tidak perlu memilih.

- `writer.Deps` memisah `Read`/`ReadEngine` dan `Write`/`WriteEngine`.
  `Write` kosong = kedua tahap memakai mesin yang sama (jalan paling umum).
- API: `POST /api/posts` menerima `write_engine` + `write_model`.
- CLI: `-write-engine` + `-write-model`.
- GUI: pemilih kedua muncul di balik centang "Pakai mesin lain untuk menulis",
  masing-masing diberi label tahapnya.
- Ringkasan waktu ikut menyebut kedua mesin — tanpa itu, "17 menit" tidak bisa
  dibandingkan dengan percobaan lain sama sekali.

### Satu bug tata letak yang ikut ketahuan

Pemilih kedua membuat kolom kanan lebih tinggi daripada jendela, dan flexbox
menyusutkan panel daftar berita lebih dulu — sampai daftarnya hilang sama sekali
dan tinggal judul + separuh kotak tempel. Terlihat dari POTRET, bukan dari
angka: `col over` waktu itu masih 0.

Melarangnya menyusut (`flex-shrink: 0`) justru lebih buruk — tinggi daftar itu
memang dihitung dari ruang sisa (`.feed-panel .news-list { flex: 1 }`), jadi
tanpa batas atas ia minta 2.900 px. Yang benar batas BAWAH pada panelnya
(`min-height: 240px`): daftarnya tetap terlihat, kolomnya yang bergulir.

## Halaman klip ikut memakai pemilih bersama (18 Agustus 2026)

Yang dibuang dari panel Engine halaman klip: **Mode (offline/hybrid)**, kotak
kunci Claude, pilihan model Claude, pilihan "mesin offline", dan pilihan model
lokal. Lima kendali untuk satu pertanyaan — dan kunci Claude hanya bisa diisi
dari sana, jadi pengguna yang datang lewat tab kartu berita tidak punya jalan.

Sekarang: **Whisper model** (itu transkripsi, bukan LLM — tetap di tempatnya)
lalu `<EnginePicker>` yang sama persis dengan tab lain, ditambah satu pilihan
`heuristic` lewat prop `extra` (bukan mesin LLM, jadi kotak model dan tombol uji
ikut hilang saat ia dipilih).

**Mode dibuang seluruhnya**, bukan disembunyikan. Ia tinggal punya dua tugas:
menentukan nilai bawaan `Provider`, dan memilih mesin koreksi transkrip. Yang
pertama sekarang dijawab pemilih mesin; yang kedua memakai mesin yang sama
dengan pemilihan momen, dengan satu kekecualian — koreksi tetap butuh LLM walau
skornya heuristik, dan di situ Ollama yang dipakai. Setelan yang cuma
menentukan nilai bawaan setelan lain adalah satu cara lagi untuk tidak sinkron.

### Pipeline tidak menyimpan tabel penyedia kedua

`api.fillEngine` mengisi `Options.LLMBase`, `LLMPath`, `LLMKeyEnv`, dan
`EngineName` dari daftar mesin yang sama; pipeline merakit kliennya dari
koordinat itu (`cloudClient`). Keempatnya `json:"-"` dan itu **bukan
kerapian**: `LLMKeyEnv` menyebut NAMA variabel lingkungan, jadi membiarkannya
datang dari JSON permintaan berarti membiarkan pemanggil membaca variabel apa
pun lalu mengirim isinya ke alamat yang juga ia pilih (`LLMBase`). Kuncinya
sendiri tidak pernah masuk `Options` — Options ikut tersimpan di riwayat job.

### Tombol Test mesin lokal menjalankan uji dua tahap

`/api/engines/test` untuk `kindLocal` memanggil `pipeline.SelfTest`, bukan
sapaan `{"ok":true}`. Alasannya temuan lama yang melahirkan uji itu: berkali-kali
di komputer baru Ollama jalan, model terpasang, sapaan berhasil — dan job klip
tetap berhenti. Sapaan menguji servernya menjawab, bukan modelnya sanggup.

### Model ikut berganti saat mesin berganti

Bug yang dilaporkan pemilik proyek: memilih DeepSeek sementara kotak model masih
berisi `llama3.1` dari job sebelumnya, lalu permintaan BERBAYAR dikirim untuk
model yang tidak ada di sana. `EnginePicker` sekarang menimpa kotak model saat
mesinnya berganti — dengan `useRef`, supaya yang ditimpa hanya pergantian
mesin, bukan render pertama halaman (di situ model tersimpan dari sesi
sebelumnya harus dibiarkan).
