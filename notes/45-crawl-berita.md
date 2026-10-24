# 45 — Crawl berita: Serper + SociaVault, hemat kredit

Rencana & pengerjaan, 25 Oktober 2026. Dikerjakan & diuji LOKAL (belum di VPS):
kueri `berita terkini, ekonomi, olahraga, teknologi, polisi`, 10 terbaru per
kueri. Kode: `news/crawl.go` (+ uji), `api/crawl.go`, tab "News search" di
Requirements (`requirements/crawl.tsx`). Uji nyata: satu putaran = 5 kredit
Serper, 50 berita, SociaVault tidak tersentuh.

## Permintaan pemilik

- Yang dicari **berita terbaru, hanya news** (bukan postingan sosmed), dibatasi
  beberapa berita per putaran.
- Hasil yang sudah diambil **disimpan lokal** dan **hilang sendiri setelah 1
  minggu**.
- Crawl **tiap 3 jam**, TAPI **hanya selama website dibuka**: tidak ada
  pengunjung = tidak ada crawl = tidak ada kredit terpakai.
- Yang dihemat: **kredit Serper/SociaVault**. AI tetap hanya dipanggil saat
  pengguna menekan tombol (ringkas, caption, isi carousel) — crawl tidak
  memanggil LLM sama sekali.
- Kredit habis / kunci ditolak / API gagal → **notifikasi kecil di halaman
  Requirements**.

## Yang sudah ada (dipakai ulang, bukan diganti)

- `news.ListAll` — RSS media Indonesia (`DefaultSources`), gratis.
- `news.Search` — pencarian Google News lewat RSS-nya, gratis.
- `news.FetchArticle` / `FetchContent` — baca artikel; dipakai saat pengguna
  MEMILIH berita, bukan saat crawl.

## Kedua API (diperiksa 25 Oktober 2026, kunci pemilik)

| | Serper | SociaVault |
| --- | --- | --- |
| Pakai | `POST https://google.serper.dev/news` | `GET https://api.sociavault.com/v1/scrape/google/search` |
| Kunci | header `X-API-KEY` | header `X-API-Key` |
| Isi | Google **News**: title, link, snippet, date, source, imageUrl | Google **Search** umum: title, url, description (tanpa gambar & sumber) |
| Saring waktu | `tbs=qdr:h` / `qdr:d` | `date_posted=last-hour` / `last-day` |
| Biaya | 1 kredit per permintaan (≤10 hasil) | 1 kredit per halaman |
| Cek saldo | `GET /account` → `{"balance":48879,"rateLimit":50}` | `GET /v1/credits` → `{"credits":14601,…}` |

Cek saldo tidak memakai kredit (sudah dicoba). Karena Serper memberi hasil
**news** lengkap dengan gambar & sumber, **Serper = utama, SociaVault =
cadangan** hanya saat Serper gagal/habis.

## Rancangan

### 1. Pemicu: "lazy", bukan jam

Tidak ada ticker di engine. Setiap kali GUI membuka daftar berita (atau
heartbeat ringan dari halaman yang terbuka), engine memeriksa
`lastCrawl`; bila sudah > 3 jam → crawl dimulai **di latar**
(`context.Background()`, aturan CLAUDE.md), jawaban langsung dari simpanan.
Satu kunci (mutex) supaya dua pengunjung tidak memicu dua crawl.

Akibatnya: server yang tidak dikunjungi seminggu memakai 0 kredit.

### 2. Urutan sumber (kredit seminimal mungkin)

1. RSS bawaan (gratis) — selalu.
2. Serper `/news`, `gl=id`, `hl=id`, `tbs=qdr:w,sbd:1`, `num=10` — **K kueri**
   per putaran (daftar topik tetap, lihat "Perlu keputusan").
3. SociaVault — HANYA bila langkah 2 gagal (kredit habis, 401, 5xx), dengan
   kueri yang sama dan `date_posted=last-day`.

Hitungan kasar dengan K = 4: paling banyak 8 putaran/hari × 4 = **32
kredit/hari** (± 1.000/bulan) — dan hanya pada hari ada yang membuka website.

### 3. Simpanan lokal

Satu berkas `<DataDir>/news/crawl.json` (bukan path dari root proyek):
`{ last_crawl, items: [{url, title, source, image, date, found_at, via}] }`.

- Kembar dibuang lewat URL yang dinormalkan (tanpa query pelacak, tanpa `/amp`).
- Item dengan `found_at` > 7 hari dibuang setiap kali berkas ditulis.
- Batas total (mis. 300 item) supaya berkas tidak tumbuh tanpa ujung.
- Di mode web satu berkas untuk seluruh tim — crawl satu kali dipakai semua.

### 4. Kunci & status

- Kunci disimpan di `.env` seperti kunci LLM: `SERPER_API_KEY`,
  `SOCIAVAULT_API_KEY`, diisi lewat halaman Requirements. **Tidak pernah
  di-commit, tidak ditulis ke notes.**
- `crawlStatus` per API di memori + ditulis ke `crawl.json`:
  `{ ok, balance, checked_at, last_error }`. Saldo diperbarui saat crawl
  (gratis).
- Kode galat → pesan: 400 "Not enough credits" / 402 → *kredit habis*;
  401/403 → *kunci ditolak*; 429 → *batas laju, dicoba putaran berikut*;
  5xx/timeout → *layanan tidak terjangkau*. Tidak di-retry berulang
  (httpx.Retry sekali jalan sudah cukup) dan TIDAK fallback diam-diam ke
  mesin lain selain urutan di §2 yang memang dirancang (notes/12).

### 5. Halaman Requirements

Satu kartu "News search": baris Serper & SociaVault — kolom kunci, saldo,
waktu crawl terakhir, dan **lencana kecil** (hijau OK / kuning cadangan
dipakai / merah habis-ditolak) dengan satu kalimat sebabnya. Tidak ada toast
di halaman lain.

### 6. Daftar News

Daftar "Semua" = RSS + hasil crawl, urut terbaru, label kecil sumbernya.
Pengguna memilih berita → alur yang sudah ada (FetchArticle, kartu, AI saat
tombol ditekan).

### 7. API & izin

- `GET /api/news/crawl/status` (Requirements) dan pemicu lazy di
  `/api/news/list` — keduanya wajib masuk `routePerms` (access.go).
- `POST /api/settings/keys` yang sudah ada diperluas untuk dua kunci baru.

### 8. Uji

- Unit: normalisasi URL & dedup, kedaluwarsa 7 hari, pemetaan kode galat →
  status, pemicu 3 jam (jam disuntikkan).
- Ujung-ke-ujung: satu crawl nyata dengan K=1 (1 kredit), cek saldo turun 1.

## Keputusan pemilik

1. Kueri: `berita terkini`, `ekonomi`, `olahraga`, `teknologi`, `polisi` (K = 5).
2. 10 berita terbaru per kategori, rentang hari ini sampai 7 hari lalu
   (Serper `tbs=qdr:w,sbd:1`, SociaVault `date_posted=last-week`), total maks 300.
3. Uji lokal dulu, baru VPS.
