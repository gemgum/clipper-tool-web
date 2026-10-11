# Cara Hitung Biaya — Claude vs OpusClip/CapCut

## Perbedaan model penghitungan

| Layanan | Dasar hitung | Sifat |
|---|---|---|
| OpusClip | Kredit per video | Produk jadi (ada margin) |
| CapCut long-to-short | Per menit video ASLI | Produk jadi |
| **Punya kita (Claude API)** | **Per token (jumlah teks transkrip)** | Bahan mentah (grosir) |

### Poin inti
Claude hanya membaca **teks transkrip**, BUKAN video.
→ Biaya tergantung **seberapa banyak dialog**, bukan durasi mentah.
- Podcast 1 jam ngobrol terus = banyak token = agak mahal.
- Video 1 jam musik/gameplay tanpa dialog = sedikit token = nyaris gratis.

CapCut tetap charge per menit walau video diam. Claude tidak.

## Konversi ke "per menit" (untuk banding dgn CapCut)

Asumsi: bicara ~150 kata/menit ≈ ~200 token/menit transkrip.
Input ~250 token/menit (transkrip + overhead), output ~33 token/menit (skor).

| Model | /menit video asli | /jam | Rp/menit (kurs 16rb) |
|---|---|---|---|
| Opus 4.8 | ~$0.002 | ~$0.13 | ± Rp 30 |
| Haiku 4.5 | ~$0.0004 | ~$0.025 | ± Rp 6 |

## Total biaya = transkripsi + scoring

| Komponen | Offline | API |
|---|---|---|
| Transkripsi (Whisper) | GRATIS (CPU/GPU sendiri) — bagian yg benar2 "per menit audio" | STT API ~$0.006/mnt = $0.36/jam (MAHAL) |
| Scoring viral | GRATIS (heuristik/Ollama) | Claude: Rp 6–30/menit |

## Kesimpulan
- Komponen "per menit video asli" ala CapCut = sebenarnya **transkripsi**.
  Jalankan Whisper offline → GRATIS (cuma listrik + waktu proses).
- Yang berbayar hanya **scoring Claude**, dihitung per token, sangat murah.
- Beda mendasar: OpusClip/CapCut jual paket jadi + margin; kita bayar biaya
  mentah token/compute yang jauh lebih kecil.

## Cara tekan biaya scoring
1. Saring dulu dengan heuristik → kirim finalis saja ke LLM, bukan transkrip penuh.
2. Prompt caching untuk instruksi penilaian yang sama (~0.1x harga).
3. Haiku untuk menyaring, Opus hanya untuk ranking akhir.
