package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/gemgum/clipper/engine/internal/correct"
	"github.com/gemgum/clipper/engine/internal/types"
)

// SelfTestStep hasil satu tahap uji. Dilaporkan per tahap, bukan diringkas jadi
// satu "berhasil/gagal": yang menolong justru TAHAP MANA yang gagal — model yang
// sanggup mengoreksi transkrip tapi tidak sanggup memilih momen adalah keadaan
// yang berbeda dari model yang tidak termuat sama sekali.
type SelfTestStep struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	Error  string `json:"error"`
	MS     int64  `json:"ms"`
}

// Transkrip contoh: sengaja dibuat menyerupai keluaran mentah whisper — tanda
// hubung dialog, tanda baca salah tempat, spasi ganda, dan satu nama daerah yang
// salah dengar ("Londo Irang"). Ketiganya persis yang harus dibenahi tahap
// koreksi, jadi model yang membalas apa adanya ikut ketahuan.
var selfTestTranscript = types.Transcript{
	Language: "id",
	Segments: []types.TranscriptSegment{
		{Start: 0, End: 4.2, Text: "- jadi gini bang, waktu itu saya sempat ketemu londo irang di pasar"},
		{Start: 4.2, End: 9.6, Text: "dan dia bilang kalau usahanya hampir bangkrut. tapi, dia nggak nyerah"},
		{Start: 9.6, End: 15.1, Text: "tiga tahun kemudian  omsetnya naik sepuluh kali lipat gila sih"},
	},
}

// Istilah contoh: memaksa jalur -terms ikut diuji. Tanpa ini tahap koreksi diuji
// setengah — daftar istilah punya prompt & pagar pengamannya sendiri.
var selfTestTerms = []string{"Londo Ireng"}

// Kandidat contoh untuk tahap pemilihan momen. Dua, bukan satu: model yang
// mengarang nomor di luar daftar hanya ketahuan bila ada nomor yang bisa salah.
var selfTestCandidates = []types.Candidate{
	{Start: 0, End: 44, Text: "Saya mulai usaha ini tahun 2019 dengan modal dua juta rupiah. " +
		"Enam bulan pertama tidak ada satu pun pembeli yang datang dua kali. " +
		"Saya hampir menutupnya, sampai seorang pelanggan bilang produknya bagus tapi kemasannya membuat orang ragu."},
	{Start: 44, End: 92, Text: "Kemasannya saya ganti total, dan bulan itu juga penjualannya naik tiga kali lipat. " +
		"Ternyata yang salah bukan produknya, melainkan hal pertama yang dilihat orang. " +
		"Sekarang saya selalu bilang ke teman-teman: perbaiki dulu yang paling dulu terlihat."},
}

// SelfTest membuktikan model lokal sanggup mengerjakan PEKERJAAN SUNGGUHAN,
// bukan sekadar membalas sapaan.
//
// Ada karena laporan yang berulang: di komputer baru, Ollama jalan, qwen2.5
// terpasang, tombol uji hijau — dan job klip tetap berhenti. Sapaan satu kata
// memang membuktikan model termuat, tapi tidak satu pun hal yang benar-benar
// menggagalkan job: jendela konteks yang terlalu kecil, balasan JSON yang tidak
// sesuai skema, dan nomor kandidat yang dikarang.
//
// Karena itu yang dijalankan di sini bukan prompt tiruan melainkan DUA TAHAP
// LLM yang sama persis dengan yang dipakai pipeline, lewat klien yang dibangun
// resolveOllama yang sama pula — dulu tombol ujinya memakai klien lain, tanpa
// Kind dan tanpa NumCtx model, dan itulah kenapa hasilnya bisa berbeda dari job.
func SelfTest(ctx context.Context, url, model string) (string, []SelfTestStep) {
	start := time.Now()
	c, name, err := resolveOllama(ctx, url, model)
	steps := []SelfTestStep{{Name: "connect", MS: time.Since(start).Milliseconds()}}
	if err != nil {
		steps[0].Error = err.Error()
		return "", steps
	}
	steps[0].OK, steps[0].Detail = true, name

	// --- tahap 1: koreksi transkrip ---
	c.Temperature = correctionTemperature
	start = time.Now()
	_, report, err := correct.Correct(ctx, selfTestTranscript, selfTestTerms,
		func(ctx context.Context, system, user string, schema any) (string, error) {
			return c.Complete(ctx, system, user, schema, 4096)
		}, name, nil)
	step := SelfTestStep{Name: "transcript correction", MS: time.Since(start).Milliseconds()}
	switch {
	case err != nil:
		step.Error = err.Error()
	case report.Missing > 0:
		// Skema mengunci jumlah entri balasan ke jumlah segmen yang dikirim, jadi
		// entri yang hilang berarti model tidak sanggup memenuhinya — di
		// transkrip sungguhan itu muncul sebagai kalimat yang tidak pernah
		// terkoreksi, bukan sebagai galat.
		step.Error = fmt.Sprintf("%s answered, but left %d of %d segments unanswered: the model is most likely too small for this task (llama3.1 8B is the smallest that handles it reliably here)",
			name, report.Missing, report.Total)
	default:
		step.OK, step.Detail = true, report.Summary()
	}
	steps = append(steps, step)

	// --- tahap 2: pemilihan momen ---
	c.Temperature = 0 // kembali ke suhu bawaan, sama seperti job
	start = time.Now()
	picks, err := c.PickMoments(ctx, selfTestCandidates, 0, 2, "id")
	step = SelfTestStep{Name: "moment selection", MS: time.Since(start).Milliseconds()}
	switch {
	case err != nil:
		step.Error = err.Error()
	case len(picks) == 0:
		step.Error = fmt.Sprintf("%s picked no moment out of %d candidates: a job would finish with zero clips", name, len(selfTestCandidates))
	case picks[0].Index < 0 || picks[0].Index >= len(selfTestCandidates):
		// Nomor di luar daftar = model mengarang alih-alih memilih. Di job
		// sungguhan pilihan itu dibuang diam-diam, jadi gejalanya cuma "klipnya
		// lebih sedikit dari yang diminta".
		step.Error = fmt.Sprintf("%s returned candidate number %d, but only 0-%d exist: it is inventing numbers instead of picking",
			name, picks[0].Index, len(selfTestCandidates)-1)
	default:
		step.OK = true
		step.Detail = fmt.Sprintf("picked #%d, score %.0f: %q", picks[0].Index, picks[0].Score, picks[0].Title)
	}
	return name, append(steps, step)
}
