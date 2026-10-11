# Job Latar Belakang: Reconnect, Minimize, Survive-Close, Antrian

Rencana fitur (BELUM diimplementasi — dicatat untuk nanti). Diskusi 27 Jul 2026.

## Fakta arsitektur saat ini (penting)

- **Job jalan di ENGINE (proses Go terpisah), bukan di browser.** Browser hanya
  menampilkan progress via SSE.
- Konsekuensi: **tutup browser TIDAK menghentikan job** — engine tetap memproses.
  Fitur "survive close" secara engine SUDAH ada; yang kurang hanya UI reconnect.
- **Belum ada antrian**: tiap POST /api/jobs langsung jalan (goroutine). Dua job
  bersamaan → jalan paralel.

## Pertanyaan yang dijawab
- New tab + run lagi → bisa? **Ya**, engine terima job kedua, jalan paralel.
- Progress lambat? **Ya** — transkripsi Whisper pakai semua core (runtime.NumCPU).
  Dua job transkripsi bersamaan → rebutan CPU → keduanya melambat (hasil tetap benar).

## Metode komunikasi — KEPUTUSAN: tetap SSE

| Metode | Untuk | Verdict |
|---|---|---|
| SSE (dipakai) | server→klien satu arah (progress), auto-reconnect bawaan | ✅ pilih ini |
| WebSocket | dua arah realtime (kontrol/steer dari klien) | ⏳ nanti bila perlu pause/steer |
| Webhook | server→server / notifikasi eksternal | ❌ bukan untuk UI browser |

SSE sudah punya replay event terminal (error/done/canceled saat reconnect).

## Yang perlu ditambah untuk fitur ini (urут)

1. **UI daftar job berjalan + reconnect**
   - GUI panggil `GET /api/jobs`, tampilkan job aktif, klik → buka SSE lagi
     (`GET /api/jobs/{id}/events`) untuk lanjut lihat progress.
   - "Minimize" = murni UI (collapse kartu job); job tetap jalan di engine.
2. **Persistensi job** (agar tahan restart engine)
   - Sekarang job in-memory → hilang bila engine dimatikan/restart.
   - Simpan state job + daftar klip ke disk (JSON/SQLite di data/).
   - Saat start, muat ulang job selesai; job yang "running" saat crash ditandai
     terputus (bisa di-resume atau tandai gagal).
3. **Antrian / batasi konkurensi** (hindari rebutan CPU)
   - Opsi A: antrian FIFO, proses 1 job sekaligus (paling sederhana & stabil).
   - Opsi B: batasi N job paralel + bagi thread Whisper (mis. NumCPU/N).
   - Rekomendasi awal: **antrian 1-per-1** untuk video panjang (1–4 jam).
4. (Opsional) **Notifikasi selesai** — desktop notification saat job kelar.
   Jika mau lintas-aplikasi, di sinilah webhook lokal bisa dipertimbangkan.

## Catatan implementasi (nanti)
- Endpoint sudah ada: GET /api/jobs (list), GET /api/jobs/{id} (status),
  GET /api/jobs/{id}/events (SSE). Reconnect tinggal dipakai di GUI.
- Untuk antrian: bungkus Manager.Create agar masuk queue, worker pool eksekusi.
- Jangan ubah kontrak API yang sudah jalan; tambahkan saja.
