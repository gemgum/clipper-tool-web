# Status MVP

Tanggal: 27 Juli 2026. **MVP selesai & terverifikasi jalan.**

## Sudah jalan (diuji end-to-end)

- [x] Scaffold proyek (engine Go, worker C++, gui Next.js) + go.work + .gitignore
- [x] `setup.sh`: build whisper.cpp dari source + unduh model + build worker
- [x] Engine Go build → `bin/clipper` (CLI + server), tanpa dependency eksternal
- [x] Worker C++ build → `bin/clipper-worker`, perintah `features` (RMS) teruji
- [x] Pipeline: ekstrak audio → features → transkripsi → segmentasi → skor → render
- [x] Output klip **1080x1920 (9:16)** + subtitle **ter-burn** (.ass)
- [x] Scoring heuristik (hook/emosi/clarity/shareability/standalone)
- [x] HTTP API + SSE progress (POST /api/jobs, GET events, clips, file)
- [x] GUI Next.js: form → progress realtime → daftar klip + preview + unduh
- [x] Integrasi engine+GUI diverifikasi (health OK, halaman tersaji)

## Cara pakai singkat

```bash
./setup.sh base          # sekali (butuh internet)
./build.sh
# CLI:
./bin/clipper run video.mp4 -model base -max 5
# atau GUI:
./bin/clipper serve &        # terminal 1
cd gui && npm run dev        # terminal 2 → buka http://localhost:3000
```

## Diverifikasi dengan
Video uji: sampel suara (JFK) di-loop jadi 150 detik → 3 klip 9:16 bersubtitle,
skor 54. (Bahasa Inggris hanya untuk uji mekanik; produksi = Indonesia.)

## Update ronde 2 (feedback pengguna) — SELESAI & teruji

- [x] **Progress akurat**: transkripsi diparse langsung dari whisper (-pp),
      dipetakan ke pita 0.20–0.53 (bagian paling lama). Teruji naik mulus.
- [x] **Error muncul di GUI**: SSE kini mengirim event terminal (error/done/
      canceled) walau job selesai sebelum GUI berlangganan. (Dulu "progress 0 diam".)
- [x] **Tombol Batalkan (cancel)**: endpoint + tombol GUI; status → canceled,
      subprocess (ffmpeg/whisper) ikut dimatikan via context.
- [x] **Daftar model** `/api/models`: GUI tahu model mana yang sudah diunduh
      (✓ siap / ✗ belum), tombol proses dikunci bila model belum ada + petunjuk.
- [x] **Drag & drop**: dropzone upload video (streaming ke disk) → path terisi
      otomatis. Endpoint POST /api/upload. Tetap bisa tempel path manual.
- [x] **Panel Log** di GUI: semua event/pesan dengan timestamp.
- [x] **Label diperjelas**: "Jumlah klip maksimum" + tooltip.
- [x] **Pilih folder output**: opsi `output_dir` (CLI `-out`, field di GUI).
      Klip final tersimpan ke folder pilihan; file sementara tetap di data/.

### Penjelasan model whisper
Model = file .bin (otak pengenal suara), diunduh sekali via `./setup.sh <nama>`.
Ukuran: tiny ~75MB, base ~142MB, small ~466MB, medium ~1.5GB, large-v3 ~2.9GB.
Makin besar = makin akurat (penting untuk Indonesia) tapi makin lambat & besar.

### Catatan drag & drop
Browser tidak memberi path asli file lokal (keamanan), jadi drag/drop
= UNGGAH file ke engine (disalin ke data/uploads). Untuk video 1–4 jam (GB),
ini menyalin file. Alternatif hemat: tempel path manual (tanpa salin).
Path asli langsung baru mungkin bila nanti dibungkus desktop (Tauri).

## Update ronde 3 (kualitas & subtitle) — SELESAI & teruji

- [x] **Resolusi**: 720p / 1080p / 1440p (opsi). Default 1080p Full HD.
- [x] **Kualitas encode**: draft / hd / max (crf+preset). Default HD (crf20/medium)
      + pix_fmt yuv420p + audio 160k → jauh lebih tajam dari sebelumnya.
- [x] **Preset durasi**: auto (30d–3mnt), 30/60/90/120/180 detik → target min/max.
- [x] **Segmentasi diperbaiki**: potong di batas kalimat; bila lewat batas atas,
      mundur ke akhir kalimat & bawa sisa ke klip berikut (tak lagi kepotong tengah).
- [x] **Subtitle lengkap**: font (Montserrat default + Anton + Bebas Neue),
      ukuran, warna (putih/kuning), posisi X/Y via \pos + \an5. fontsdir ke libass.
- [x] **Endpoint baru**: /api/fonts, /api/probe, /api/frame (preview 9:16 JPEG).
- [x] **GUI**: pilih resolusi/kualitas/durasi + panel subtitle dengan
      **preview frame** dan **teks subtitle bisa digeser** (mouse & sentuh),
      slider ukuran, slider waktu frame.
- [x] Font diunduh via setup.sh (langkah 4/4).

Teruji: render 1080p HD + Montserrat → output 1080x1920, endpoint frame 720x1280.

## Update ronde 4 (fitur 1-6 + review kode) — SELESAI & teruji

- [x] **Review kode**: `go vet` menemukan bug laten (Job menyalin sync.Mutex saat
      Snapshot) → diperbaiki dengan tipe `JobView` terpisah. Vet kini bersih.
- [x] **(1) Subtitle karaoke per-kata**: opsi `karaoke` → tag \k di .ass, durasi
      per kata dibagi rata (aktif=warna sorot, belum=putih). Teruji ada {\k..}.
- [x] **(2) Font asli di preview**: endpoint /api/font-file + @font-face di GUI.
- [x] **(4) Outline & kotak**: opsi `outline` (ketebalan), `box` (latar kotak).
- [x] **(5) Simpan preset**: setelan disimpan di localStorage, dimuat otomatis.
- [x] **(6) Antrian job**: manager proses 1 job/waktu (flag serve -jobs N).
      Teruji: job kedua 'queued' selagi job pertama jalan. Cancel job antri jalan.
- [x] **(3) Preview**: sudah pakai crop 9:16 dari sumber + slider waktu frame
      (versi penuh "dari klip nyata" menyusul setelah proses).
- [x] Robust fetch di GUI: respons non-JSON (mis. engine lama) diberi pesan jelas.

### Penyebab error "Unexpected non-whitespace ... position 4"
Engine yang berjalan masih binary LAMA (belum ada /api/probe) → balas teks
"404 page not found" (bukan JSON). **Solusi: hentikan lalu jalankan ulang engine**
(rebuild sudah dilakukan). GUI kini juga memberi pesan jelas bila ini terjadi.

## Update ronde 5 (wrap, folder unik, reconnect) — SELESAI & teruji

- [x] **Wrap subtitle**: teks panjang dibungkus otomatis (center, maks 3 baris),
      kelebihan dipecah jadi halaman berurutan dalam rentang waktu segmen.
      Lebar dihitung dari ukuran font. Teruji: 1 kalimat → 3 baris + halaman lanjut.
- [x] **Folder unik**: nama `<job_id>_<YYYY-MM-DD_HH-MM-SS>` untuk workDir & output.
      Tak tertimpa walau engine restart (job_id berulang jadi job_0001). Field `dir`
      diekspos di API.
- [x] **Subfolder di output custom**: klip masuk `<output-mu>/<job_id>_tanggal_jam/`.
- [x] **Reconnect saat tab reload/baru**: GUI saat load memanggil /api/jobs,
      menyambung ke job status running/queued (progress + klip yang sudah ada
      tampil lagi). Teruji: /api/jobs menampilkan job 'running'.
- [x] **Riwayat**: DIBATALKAN sesuai permintaan (fokus ke reconnect saja).

## Update ronde 6 (ketajaman & FPS) — SELESAI & teruji

- [x] **Masalah gambar pecah**: penyebab = reframe crop (zoom) → strip tengah
      sumber dibesarkan. Solusi 2 lapis:
  - Scaling **lanczos** (lebih tajam saat membesarkan) pada semua mode.
  - Mode reframe **"fit" (latar blur)**: frame utuh di atas latar blur, TANPA
      zoom → paling tajam. Filtergraph: split → bg(blur) + fg(fit) → overlay.
- [x] **Setting FPS**: opsi ikut-sumber / 24 / 30 / 60 (filter fps). Catatan: FPS
      = kehalusan gerak, bukan ketajaman.
- [x] CLI: flag baru `-reframe fit|center`, `-fps`, `-resolution`, `-quality`.
- [x] GUI: selektor "Cara pas ke 9:16" (isi penuh/fit) + "FPS"; masuk preset.
- [x] #3 (konfirmasi video sama) DIBATALKAN sesuai permintaan.

Teruji: fit+fps 720p→720x1280@24fps & 1080p→1080x1920@30fps (CLI & API).
Untuk hasil paling tajam pada video landscape: pakai mode **fit**.

## Update ronde 7 (redesign scoring LLM) — Fase 1 SELESAI (perlu key untuk uji)

- [x] **Folder di-rename** clipper → clipper-tool (ada backup "clipper - Copy").
      Semua kerja lanjut di clipper-tool.
- [x] **Fix portabilitas whisper**: rename mematahkan RUNPATH absolut binary
      (libwhisper.so tak ketemu). Solusi: salin *.so ke bin/ + engine set
      LD_LIBRARY_PATH ke folder binary. setup.sh diperbarui. Tahan pindah folder
      & penting untuk distribusi. Teruji: offline jalan lagi.
- [x] **Fase 1 — LLM pilih batas momen** (redesign scoring):
  - Fungsi `SelectMoments`: kirim transkrip BERTIMESTAMP → LLM tentukan start/end
    tiap momen (durasi BERVARIASI, bukan window tetap). Menjawab "selalu 30 detik".
  - Preset durasi jadi PANDUAN di prompt (boleh menyimpang demi momen utuh).
  - Batas LLM dirapikan ke segmen terdekat (`snapToSegments`).
  - Heuristik jadi FALLBACK bila tak ada API key.
  - Build & vet bersih. **Perlu ANTHROPIC_API_KEY + mode hybrid untuk uji jalur LLM.**

### Cara uji Fase 1
1. Isi `ANTHROPIC_API_KEY=...` di `.env`.
2. Di GUI pilih mode **hybrid** (atau CLI: `-mode hybrid`).
3. (Kualitas: bisa set model lebih kuat via `llm_model`, mis. claude-sonnet-5.)

### Berikutnya
- Fase 2: GUI atur API key + pilih/ganti model LLM dinamis.
- Fase 3: integrasi Ollama/Qwen (deteksi, unduh dari app, notifikasi, gonta-ganti).

## Update ronde 8 (Fase 2+3: UI mode-adaptif + Ollama) — SELESAI

- [x] **UI berubah menurut mode**:
  - hybrid → input **API Key Claude** + tombol Simpan + pilih model (Haiku/Sonnet/Opus).
  - offline → pilih mesin (Ollama / Heuristik) + model lokal (Qwen/Llama/Gemma) +
    status Ollama + tombol **unduh model** + notifikasi bila Ollama tak ada/gagal.
- [x] **Backend Ollama** (`score/ollama`): SelectMoments via /api/chat (format json),
  `Status()` (deteksi jalan + daftar model), `Pull()` (unduh model).
- [x] **Provider dinamis** di pipeline: claude | ollama | heuristik (interface
  `momentSelector`; keduanya SelectMoments). Fallback heuristik bila LLM gagal.
- [x] **API key dari GUI**: POST /api/settings → simpan di memori + tulis .env
  (menimpa env). GET /api/settings → has_key.
- [x] Endpoint: /api/settings (GET/POST), /api/ollama/status, /api/ollama/pull.
- [x] Config: field `provider`, `ollama_model`, `ollama_url`. Preset LLM disimpan
  di localStorage (kecuali key — key di server).

Teruji: settings simpan/baca, ollama status (running:false anggun), pull gagal
→ notifikasi. Build engine+GUI bersih, vet bersih.
**Belum teruji live:** jalur Claude (perlu key asli) & Ollama (perlu install +
model). Plumbing + fallback terverifikasi.

### Catatan LLM lokal (Ollama)
- Qwen2.5 disarankan untuk Indonesia. Di CPU: lambat, num_ctx dibatasi 8192 →
  video panjang perlu chunking (penyempurnaan berikutnya).

## Update ronde 9 (Ollama live + fix) — TERVALIDASI

- [x] **Fix CMake rename**: build dir CMake simpan path absolut → rename folder
      bikin error. build.sh/setup.sh kini CLEAN build worker; setup bersihkan
      cache whisper hanya bila basi. Kebal pindah folder.
- [x] **GUI model Ollama dinamis**: dropdown tampilkan model yang benar-benar
      terpasang (mis. qwen:latest) + auto-pilih; opsi lain ditandai "perlu unduh".
- [x] **Fix bug Ollama #1**: model lokal balas skor desimal ("score":7.0) →
      struct int crash. Moment kini float64, dikonversi int saat jadi klip.
- [x] **Fix bug Ollama #2**: prompt kompleks (reasons+hashtags) bikin model lemah
      KEBABLASAN mengulang → JSON rusak. Prompt Ollama disederhanakan
      (start/end/score/title saja) + num_predict cap. Reasons diratakan dari skor.
- [x] **TERVALIDASI LIVE**: offline+Ollama(qwen) → memilih momen (judul+skor),
      bukan heuristik. Seluruh pipeline LLM-driven bekerja dgn model lokal.

### Catatan kualitas
- `qwen:latest` = Qwen 1.x (lama & lemah) → hasil terbatas (kadang halusinasi,
  durasi tak sesuai panduan). **Saran: `ollama pull qwen2.5`** (jauh lebih baik
  untuk Indonesia & ikuti instruksi). Uji dgn konten Indonesia asli.
- Video panjang tetap perlu chunking (num_ctx 8192) — penyempurnaan berikut.

## Update ronde 10 (perbaikan v2 dari uji pakai) — SELESAI & teruji

Rincian & akar masalah: `notes/11-perbaikan-v2.md`.

- [x] **Timestamp per kata**: whisper dipanggil `-ojf`, token digabung jadi kata
      + waktunya (`TranscriptSegment.Words`). Fallback bagi rata bila kosong.
- [x] **Durasi klip tak lagi mentok 30 detik**: segmentasi mengincar durasi
      ideal (tengah rentang), potong di akhir kalimat / jeda diam panjang.
      Preset auto 30–180 → **45–120**. Teruji: preset 60 → klip 60/59/60 dtk.
- [x] **Warna subtitle tidak dipaksa kuning** saat efek per-kata aktif.
- [x] **Gaya subtitle jadi 3 mode**: normal · karaoke (kata aktif disorot,
      1 Dialogue per kata + `\c` inline) · word (satu kata per layar).
- [x] **Kecepatan subtitle**: tampilan dibangun dari aliran kata seluruh klip
      (bukan per segmen), durasi minimum 1,2 dtk, maks 2–3 baris, setelan
      Lambat/Normal/Padat. Teruji: tampilan terpendek 0,45 → 1,23 detik.
- [x] **Pembatas area aman sosmed** di GUI: TikTok/Reels/Shorts/Umum, arsiran
      zona tertutup UI di preview + tombol "Taruh di area aman" + peringatan.
- [x] **Opsi simpan klip**: `burn` / `clean` / `both` (+ ekspor `.srt`).
      CLI `-save`, GUI dropdown, API `?varian=polos|srt`.
- [x] **Fix**: mode offline di CLI tak lagi memanggil Claude (provider default
      kosong → ditentukan mode); flag baru `-provider`.
- [x] Uji otomatis pertama di repo: `segment_test.go`, `subtitle_test.go`.

## Update ronde 11 (kebijakan mesin skor) — SELESAI & teruji

Rincian: `notes/12-kebijakan-mesin-skor.md`.

- [x] **Tanpa fallback**: mesin pilihan pengguna dipakai apa adanya; gagal =
      job gagal dengan pesan akar masalah (key ditolak, model belum diunduh,
      Ollama mati, JSON tak terbaca, isian kosong, batas waktu ngawur).
- [x] **Prompt detail & identik** untuk Claude + Ollama; bentuk balasan Ollama
      dijamin **JSON Schema** (`format`), bukan lagi disederhanakan.
- [x] **Chunking bertumpang-tindih** (12 mnt Ollama / 25 mnt Claude, overlap 2
      mnt) + tanda `"berlanjut"` untuk menyambung momen yang terbelah batas.
      Menutup bahaya video >30 menit terpotong diam-diam oleh num_ctx.
- [x] **Cache transkrip** (hash isi video + model + bahasa): 71 dtk → 0,4 dtk.
- [x] **Kode mati dihapus**: `llm.Judge()`/`Judgment` peninggalan desain lama.
- [x] CLI: `-provider`, `-ollama-model`. Uji baru: `pipeline/chunk_test.go`.
- [!] **Temuan**: `qwen:latest` (4B) terlalu kecil untuk prompt detail — membalas
      isian kosong. Perlu `ollama pull qwen2.5` untuk jalur Ollama.

## Belum / berikutnya (urут prioritas)

1. **Model Indonesia**: unduh `small`/`medium`, uji dengan audio Indonesia asli.
2. **Mode hybrid**: uji scoring + judul/hashtag via Claude (set ANTHROPIC_API_KEY).
3. **Kalibrasi zona sosmed**: angka pembatas TikTok/Reels/Shorts masih perkiraan
   — cocokkan dengan screenshot aplikasi asli.
4. **Face-follow 9:16** di worker C++ (OpenCV) — perintah `reframe` mode face_follow.
5. **Cache transkrip** (hash video → skip transkripsi ulang).
6. **Editor subtitle** di GUI sebelum render (endpoint PATCH sudah direncanakan).
7. **Persistensi job** (sekarang in-memory; hilang saat restart).
8. **Progress transkripsi lebih halus** untuk video 1–4 jam (parse progress whisper).

## Catatan teknis
- Bug yang sudah diperbaiki: (a) urutan argumen CLI (flag setelah path),
  (b) path model per-job di server (dulu terkunci ke default).
- Whisper `base` untuk uji cepat; `small`/`medium` untuk akurasi Indonesia.
- Video 1–4 jam: transkripsi = bagian paling lama; 32 core CPU membantu.
