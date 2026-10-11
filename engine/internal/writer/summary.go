package writer

import (
	"fmt"
	"strings"
	"time"
)

// Stage satu tahap yang sudah selesai diukur.
type Stage struct {
	Label string  `json:"label"`
	Sec   float64 `json:"seconds"`
	Note  string  `json:"note,omitempty"`
}

// Summary rincian waktu satu job.
//
// Alasannya sama dengan ringkasan pipeline klip (notes: pipeline/summary.go):
// menebak "tahap mana yang lama" selalu meleset. Di sini yang dibandingkan
// antar percobaan adalah mesin LLM — 6 panggilan ke model lokal versus 6
// panggilan ke Claude bukan cuma beda harga, tapi beda menit.
type Summary struct {
	TotalSec   float64 `json:"total_seconds"`
	Sources    int     `json:"sources"`
	Violations int     `json:"violations"`
	// Mesin tiap tahap ikut dicatat: sejak keduanya bisa berbeda, "17 menit"
	// tanpa menyebut siapa yang mengerjakan apa tidak bisa dibandingkan dengan
	// percobaan lain sama sekali.
	ReadEngine  string  `json:"read_engine,omitempty"`
	WriteEngine string  `json:"write_engine,omitempty"`
	Stages      []Stage `json:"stages"`
}

type recorder struct {
	t0     time.Time
	stages []Stage
}

func newRecorder() *recorder { return &recorder{t0: time.Now()} }

func (r *recorder) since(label string, from time.Time, note string) {
	r.stages = append(r.stages, Stage{Label: label, Sec: time.Since(from).Seconds(), Note: note})
}

func (r *recorder) summary(sources, violations int) Summary {
	return Summary{
		TotalSec:   time.Since(r.t0).Seconds(),
		Sources:    sources,
		Violations: violations,
		Stages:     r.stages,
	}
}

// Format menyusun tabel monospace untuk terminal dan kotak log GUI.
func (s Summary) Format() string {
	width := 0
	for _, st := range s.Stages {
		if len(st.Label) > width {
			width = len(st.Label)
		}
	}
	var sb strings.Builder
	sb.WriteString("\nTime per stage\n")
	for _, st := range s.Stages {
		fmt.Fprintf(&sb, "  %-*s  %7s", width, st.Label, secs(st.Sec))
		if pct := share(st.Sec, s.TotalSec); pct != "" {
			fmt.Fprintf(&sb, "  %5s", pct)
		}
		if st.Note != "" {
			fmt.Fprintf(&sb, "  %s", st.Note)
		}
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "  %-*s  %7s\n", width, "TOTAL", secs(s.TotalSec))
	fmt.Fprintf(&sb, "  %d source %s, %d unverified %s\n",
		s.Sources, plural(s.Sources, "article", "articles"),
		s.Violations, plural(s.Violations, "item", "items"))
	if s.ReadEngine != "" {
		if s.WriteEngine != "" && s.WriteEngine != s.ReadEngine {
			fmt.Fprintf(&sb, "  read %s · write %s\n", s.ReadEngine, s.WriteEngine)
		} else {
			fmt.Fprintf(&sb, "  engine %s\n", s.ReadEngine)
		}
	}
	return sb.String()
}

func secs(v float64) string {
	if v >= 60 {
		return fmt.Sprintf("%dm%02ds", int(v)/60, int(v)%60)
	}
	return fmt.Sprintf("%.1fs", v)
}

func share(part, total float64) string {
	if total <= 0 {
		return ""
	}
	return fmt.Sprintf("%.0f%%", 100*part/total)
}
