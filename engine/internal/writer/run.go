package writer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/news"
)

// Progress kabar kemajuan satu job. Bentuknya mengikuti pipeline klip supaya
// lapisan job & SSE memperlakukan keduanya sama.
type Progress struct {
	Stage   string  `json:"stage"` // gathering | reading | writing | saving | done
	Value   float64 `json:"value"`
	Message string  `json:"message"`
}

// Options setelan satu job penulisan.
type Options struct {
	// URLs = isi keranjang. Dari jalan mana pun (cari, centang, tempel).
	URLs []string
	// Lang mengatur teks tetap yang ditulis engine ke berkas (bawaan "en").
	Lang string
	// MaxWords = jatah kata badan artikel yang dikirim ke model di tahap 1.
	// 0 memakai DefaultMaxWords.
	MaxWords int
}

// Completer memanggil LLM dengan skema balasan yang diminta pemanggilnya.
//
// Beda dari news.Completer yang skemanya sudah dipaku: di sini satu mesin
// melayani DUA tugas dengan bentuk balasan yang berbeda (FactsSchema di tahap
// 1, ComposeSchema di tahap 2), dan pada model lokal skema itulah yang menjaga
// balasannya tetap JSON rapi (notes/35).
type Completer func(ctx context.Context, system, user string, schema any) (string, error)

// Deps = barang dari luar yang dibutuhkan job ini.
//
// Dibuat eksplisit supaya paket writer tidak perlu mengenal config, capture,
// maupun api — perakitannya urusan lapisan atas, seperti news.Browser.
//
// Mesinnya DIPISAH per tahap, dan itu bukan kelengkapan yang dicari-cari: kedua
// tahap punya sifat yang berbeda tajam. Tahap 1 menyalin-ulang, dijalankan
// sekali per artikel (lima panggilan), dan model murah sudah cukup. Tahap 2
// menulis, cuma sekali, dan di sinilah mutu model terasa — llama3.1 berhenti di
// 84-267 kata sementara DeepSeek mencapai 505 (notes/39). Memaksa keduanya
// memakai mesin yang sama berarti membayar mahal lima kali untuk pekerjaan
// murah, atau menerima artikel buruk demi hemat.
type Deps struct {
	// Read = mesin tahap 1 (baca fakta).
	Read       Completer
	ReadEngine string
	// Write = mesin tahap 2 (menulis). Kosong berarti memakai Read.
	Write       Completer
	WriteEngine string

	Browse   news.Browser
	CacheDir string
	OutDir   string
}

// writer memilih mesin tahap 2, jatuh ke mesin tahap 1 bila tidak disetel.
func (d Deps) writer() (Completer, string) {
	if d.Write != nil {
		return d.Write, d.WriteEngine
	}
	return d.Read, d.ReadEngine
}

// stage membungkus satu pemanggil jadi news.Completer dengan skema tahapnya.
func stage(c Completer, schema any) news.Completer {
	return func(ctx context.Context, system, user string) (string, error) {
		return c(ctx, system, user, schema)
	}
}

// SourceRef satu artikel sumber, seringkas yang perlu untuk menyebutkannya.
//
// Basket.Sources membawa artikel PENUH dan sengaja tidak ikut ke JSON; ini
// bentuk yang boleh dikirim ke GUI, supaya kaki artikel di layar menyebut
// sumber yang sama persis dengan yang ditulis ke artikel.md — termasuk yang
// dibuang karena kembar atau melenceng.
type SourceRef struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Media string `json:"media"`
}

// Result hasil satu job.
type Result struct {
	Post    Post        `json:"post"`
	Draft   Draft       `json:"draft"`
	Basket  Basket      `json:"basket"`
	Sources []SourceRef `json:"sources,omitempty"`
	Summary Summary     `json:"summary"`
}

// Run menjalankan ketiga tahap berurutan: kumpulkan → baca → tulis → simpan.
//
// Satu job memanggil LLM sebanyak (jumlah artikel + 1) kali, dan pada model
// lokal itu hitungan menit — jadi pemanggilnya WAJIB menjalankannya di latar
// dengan context.Background(), bukan di dalam context permintaan HTTP
// (notes/10, dan pelajaran mahal di notes/25).
func Run(ctx context.Context, opts Options, deps Deps, onProgress func(Progress)) (Result, error) {
	rec := newRecorder()
	emit(onProgress, Progress{Stage: "gathering", Value: 0.02, Message: "Reading source articles"})

	t0 := time.Now()
	basket, err := Gather(ctx, opts.URLs, deps.Browse, deps.CacheDir, opts.Lang)
	if err != nil {
		return Result{}, err
	}
	rec.since("Gather sources", t0, plural(len(basket.Sources), "1 article", fmt.Sprintf("%d articles", len(basket.Sources))))
	for _, s := range basket.Skipped {
		emit(onProgress, Progress{Stage: "gathering", Value: 0.05, Message: "Skipped " + s.URL + ": " + s.Reason})
	}
	for _, t := range basket.OffTopic {
		emit(onProgress, Progress{Stage: "gathering", Value: 0.05,
			Message: "Warning: this article shares no keywords with the others: " + t})
	}

	// Tahap 1, satu panggilan per artikel.
	var sources []Source
	for i, content := range basket.Sources {
		emit(onProgress, Progress{
			Stage:   "reading",
			Value:   0.05 + 0.55*float64(i)/float64(len(basket.Sources)),
			Message: fmt.Sprintf("Extracting facts %d/%d: %s (%s)", i+1, len(basket.Sources), content.Article.Source, deps.ReadEngine),
		})
		t := time.Now()
		sheet, err := ExtractFacts(ctx, content, stage(deps.Read, FactsSchema()), deps.ReadEngine, opts.MaxWords)
		if err != nil {
			return Result{}, fmt.Errorf("reading %s: %w", content.Article.URL, err)
		}
		rec.since("Read "+shortName(content.Article.Source, i), t,
			fmt.Sprintf("%d facts, %d rejected", len(sheet.Facts), len(sheet.Rejected)))
		sources = append(sources, Source{Article: content, Facts: sheet})
	}

	// Tahap 2, satu panggilan.
	write, writeName := deps.writer()
	emit(onProgress, Progress{Stage: "writing", Value: 0.62, Message: "Writing the article: " + writeName})
	t := time.Now()
	draft, err := Compose(ctx, sources, stage(write, ComposeSchema()), writeName)
	if err != nil {
		return Result{}, err
	}
	note := fmt.Sprintf("%d words", draft.Words)
	if draft.Repaired {
		note += ", repaired once"
	}
	rec.since("Write article", t, note)
	for _, v := range draft.Violations {
		emit(onProgress, Progress{Stage: "writing", Value: 0.9,
			Message: fmt.Sprintf("Unverified: %s %q: %s", v.Kind, v.Text, v.Detail)})
	}

	// Tahap 3.
	emit(onProgress, Progress{Stage: "saving", Value: 0.93, Message: "Saving the article"})
	t = time.Now()
	post, err := Save(ctx, deps.OutDir, draft, sources, opts.Lang, time.Now())
	if err != nil {
		return Result{}, err
	}
	rec.since("Save files", t, post.ImageNote)

	sum := rec.summary(len(sources), len(draft.Violations))
	sum.ReadEngine, sum.WriteEngine = deps.ReadEngine, writeName
	res := Result{Post: post, Draft: draft, Basket: basket, Sources: refs(sources), Summary: sum}
	emit(onProgress, Progress{Stage: "done", Value: 1.0, Message: res.Summary.Format()})
	return res, nil
}

func refs(sources []Source) []SourceRef {
	out := make([]SourceRef, 0, len(sources))
	for _, s := range sources {
		out = append(out, SourceRef{
			Title: s.Article.Article.Title,
			URL:   s.Facts.URL,
			Media: s.Facts.Source,
		})
	}
	return out
}

func emit(onProgress func(Progress), p Progress) {
	if onProgress != nil {
		onProgress(p)
	}
}

// shortName memberi label tahap yang enak dibaca di ringkasan.
func shortName(source string, i int) string {
	if s := strings.TrimSpace(source); s != "" {
		return s
	}
	return fmt.Sprintf("source %d", i)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
