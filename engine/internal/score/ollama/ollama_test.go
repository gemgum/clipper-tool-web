package ollama

import "testing"

func TestBaseName(t *testing.T) {
	cases := map[string]string{
		"qwen2.5:latest":            "qwen2.5",
		"qwen2.5":                   "qwen2.5",
		"llama3.1:8b-instruct-q4_0": "llama3.1",
	}
	for in, want := range cases {
		if got := BaseName(in); got != want {
			t.Errorf("BaseName(%q) = %q, mau %q", in, got, want)
		}
	}
}

func TestParseParams(t *testing.T) {
	cases := map[string]float64{
		"7.6B": 7.6,
		"4B":   4,
		"270M": 0.27,
		"8x7B": 7, // MoE: yang aktif per token
		"":     0,
		"abc":  0,
	}
	for in, want := range cases {
		if got := parseParams(in); got != want {
			t.Errorf("parseParams(%q) = %v, mau %v", in, got, want)
		}
	}
}

func TestJudge(t *testing.T) {
	// Model 7B berkonteks panjang: siap.
	if ok, note := judge("qwen2.5:latest", 4_700_000_000, "7.6B", 32768, []string{"completion", "tools"}); !ok {
		t.Errorf("qwen2.5 seharusnya siap, malah ditolak: %s", note)
	}
	// Model 4B: dipakai, tapi WAJIB membawa peringatannya (notes/12 — balasannya
	// isian kosong saat memilih momen).
	if ok, note := judge("kecil:latest", 2_000_000_000, "4B", 32768, []string{"completion"}); !ok || note == "" {
		t.Errorf("model 4B seharusnya dipakai dengan peringatan, dapat ok=%v note=%q", ok, note)
	}
	// Konteks lebih kecil dari yang diminta engine: DIPAKAI, dengan catatan.
	// Potongan yang gagal dibaca dipecah sampai model sanggup (correct.Correct),
	// jadi menolaknya berarti membuang model yang sebenarnya bisa bekerja.
	if ok, note := judge("sempit:latest", 4_000_000_000, "7B", 4096, []string{"completion"}); !ok || note == "" {
		t.Errorf("konteks 4096 seharusnya dipakai dengan catatan, dapat ok=%v note=%q", ok, note)
	}
	// Terlalu kecil untuk satu segmen pun: ditolak.
	if ok, _ := judge("mini:latest", 1_000_000_000, "7B", 1024, []string{"completion"}); ok {
		t.Error("konteks 1024 seharusnya ditolak")
	}
	// Model embedding tak bisa membuat teks.
	if ok, _ := judge("embed:latest", 700_000_000, "335M", 8192, []string{"embedding"}); ok {
		t.Error("model embedding seharusnya ditolak")
	}
	// Model cloud: terdaftar seperti model biasa, berukuran 0 byte, dan
	// jalannya di server Ollama — bukan di komputer ini.
	if ok, note := judge("gpt-oss:120b-cloud", 0, "116.8B", 131072, []string{"completion"}); ok {
		t.Errorf("model cloud seharusnya ditolak, malah lolos: %s", note)
	}
	// Metadata kosong (Ollama lama): jangan menuduh, anggap siap.
	if ok, note := judge("lawas:latest", 4_000_000_000, "", 0, nil); !ok {
		t.Errorf("metadata kosong seharusnya lolos, malah: %s", note)
	}
}

// TestCtxForMuatPromptDanKeluaran menjaga temuan 18 Agustus 2026: pembuat
// berita meminta 8192 token keluaran DI DALAM jendela 8192, jadi Ollama
// berhenti di dinding konteks dan mengembalikan JSON terpotong tanpa satu pun
// pesan galat. Jendelanya harus memuat prompt DAN keluaran yang diminta.
func TestCtxForMuatPromptDanKeluaran(t *testing.T) {
	besar := &Client{NumCtx: 131072}
	if got := besar.ctxFor(9000, 8192); got <= 8192 {
		t.Errorf("ctxFor = %d — jendela tidak dilebarkan untuk prompt+keluaran", got)
	}

	// Permintaan kecil tidak menurunkan jendela di bawah angka bawaan.
	if got := besar.ctxFor(100, 512); got != numCtx {
		t.Errorf("ctxFor = %d, mau %d untuk permintaan kecil", got, numCtx)
	}

	// Model kecil tidak boleh dilampaui: meminta lebih dari yang ia sanggup
	// membuat Ollama membalas KOSONG, kegagalan yang lebih sulit dilacak.
	kecil := &Client{NumCtx: 4096}
	if got := kecil.ctxFor(9000, 8192); got != 4096 {
		t.Errorf("ctxFor = %d, mau 4096 — batas model dilampaui", got)
	}

	// Kemampuan model tidak diketahui → jangan menaikkan apa pun.
	tak := &Client{}
	if got := tak.ctxFor(9000, 8192); got != numCtx {
		t.Errorf("ctxFor = %d, mau %d saat kemampuan model tidak diketahui", got, numCtx)
	}
}
