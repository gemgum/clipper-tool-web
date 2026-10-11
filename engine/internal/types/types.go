// Package types memuat tipe domain bersama agar paket lain tidak saling impor.
package types

import "strings"

// Word satu kata dengan waktu ucap sebenarnya (detik).
type Word struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// TranscriptSegment satu potongan transkrip dengan timing (detik).
// Words diisi bila whisper memberi timestamp per token (-ojf); boleh kosong.
type TranscriptSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
	Words []Word  `json:"words,omitempty"`
}

// WordList mengembalikan kata beserta waktunya. Bila whisper tidak memberi
// timestamp per kata, waktu dibagi rata sepanjang segmen (perkiraan).
func (s TranscriptSegment) WordList() []Word {
	if len(s.Words) > 0 {
		return s.Words
	}
	fields := strings.Fields(s.Text)
	if len(fields) == 0 {
		return nil
	}
	dur := s.End - s.Start
	if dur <= 0 {
		dur = float64(len(fields)) * 0.35 // asumsi kasar bila timing rusak
	}
	per := dur / float64(len(fields))
	out := make([]Word, 0, len(fields))
	for i, w := range fields {
		out = append(out, Word{
			Start: s.Start + per*float64(i),
			End:   s.Start + per*float64(i+1),
			Text:  w,
		})
	}
	return out
}

// Transcript hasil transkripsi lengkap.
type Transcript struct {
	Language string              `json:"language"`
	Segments []TranscriptSegment `json:"segments"`
}

// Candidate kandidat klip (jendela waktu) sebelum dinilai.
type Candidate struct {
	Start float64             `json:"start"`
	End   float64             `json:"end"`
	Text  string              `json:"text"`
	Segs  []TranscriptSegment `json:"-"`
}

// Duration klip dalam detik.
func (c Candidate) Duration() float64 { return c.End - c.Start }

// Reasons rincian skor per dimensi (0-100).
type Reasons struct {
	Hook         int `json:"hook"`
	Emotion      int `json:"emotion"`
	Clarity      int `json:"clarity"`
	Shareability int `json:"shareability"`
	Standalone   int `json:"standalone"`
}

// Clip hasil akhir satu klip.
type Clip struct {
	ID         string   `json:"id"`
	JobID      string   `json:"job_id"`
	Start      float64  `json:"start"`
	End        float64  `json:"end"`
	Duration   float64  `json:"duration"`
	Score      int      `json:"score"`
	Reasons    Reasons  `json:"reasons"`
	Title      string   `json:"title"`
	Hashtags   []string `json:"hashtags"`
	Transcript string   `json:"transcript"`
	// Tidak ada subtitle_path: .ass hanya berkas antara di tmp/ yang dihapus
	// setelah dibakar. Yang bertahan adalah .srt (mode clean/both).
	VideoPath    string `json:"video_path"`     // varian utama (dibuka GUI)
	VideoPathRaw string `json:"video_path_raw"` // varian polos, bila diminta
	SubtitleSRT  string `json:"subtitle_srt"`   // .srt di folder output (opsional)
	// .txt berisi ucapan klip tanpa timestamp — untuk ditempel ke LLM lain saat
	// membuat caption. Selalu ada, apa pun mode simpannya.
	TranscriptTXT string `json:"transcript_txt"`
	Status        string `json:"status"` // scored | rendering | rendered | failed
	// Error = sebab render gagal (Status "failed"); kosong selain itu.
	Error string `json:"error,omitempty"`
}
