package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/transcribe"
	"github.com/gemgum/clipper/engine/internal/types"
)

// aiStudio = transkripsi lewat AI Studio dipilih untuk job ini (notes/43).
func (p *Pipeline) aiStudio() bool { return p.Opts.Transcriber == config.TranscriberAIStudio }

// openAIAudio = mesin transkripsi OpenAI-compatible job ini; nil bila bukan.
func (p *Pipeline) openAIAudio() *transcribe.OpenAIAudio {
	if p.Opts.Transcriber != config.TranscriberAPI {
		return nil
	}
	return &transcribe.OpenAIAudio{
		Name: p.Opts.TranscribeName, Base: p.Opts.TranscribeBase, Path: p.Opts.TranscribePath,
		Key: strings.TrimSpace(os.Getenv(p.Opts.TranscribeKeyEnv)), Model: p.Opts.TranscribeModel,
	}
}

// transcribeModel = model transkripsi "api" yang benar-benar dipakai.
func (p *Pipeline) transcribeModel() string {
	if p.Opts.TranscribeModel == "" {
		return transcribe.OpenAIDefaultModel
	}
	return p.Opts.TranscribeModel
}

// transcriberReady memeriksa mesin transkripsi yang dipilih SEBELUM apa pun
// dikerjakan: kunci yang kosong ketahuan sebelum audio diekstrak.
func (p *Pipeline) transcriberReady() error {
	if p.aiStudio() {
		return transcribe.AIStudioFromEnv().Available()
	}
	if o := p.openAIAudio(); o != nil {
		return o.Available()
	}
	return p.wh.Available()
}

// TranscriberName menyebut mesin transkripsi untuk ringkasan job.
func (p *Pipeline) TranscriberName() string {
	if p.aiStudio() {
		return "AI Studio (" + transcribe.AIStudioFromEnv().Model + ")"
	}
	if p.Opts.Transcriber == config.TranscriberAPI {
		return p.Opts.TranscribeName + " (" + p.transcribeModel() + ")"
	}
	return "whisper " + p.Opts.WhisperModel
}

// transcriberCacheModel = bagian "model" di kunci cache transkrip. Whisper
// tetap memakai nama modelnya saja, jadi cache whisper yang sudah ada tetap
// terpakai; AI Studio diberi awalan supaya keduanya tidak pernah bertabrakan.
func (p *Pipeline) transcriberCacheModel() string {
	if p.aiStudio() {
		return "aistudio:" + transcribe.AIStudioFromEnv().Model
	}
	if p.Opts.Transcriber == config.TranscriberAPI {
		return "api:" + p.Opts.TranscribeBase + ":" + p.transcribeModel()
	}
	return p.Opts.WhisperModel
}

// TranscriptResult adalah hasil tahap transkripsi beserta angka waktunya.
//
// Durasinya dikembalikan, bukan dicatat sendiri ke recorder: pemanggil yang
// tahu tahap ini bagian dari ringkasan yang mana — pipeline klip punya
// ringkasannya sendiri, pembuat caption tidak punya sama sekali.
type TranscriptResult struct {
	Transcript types.Transcript
	// CacheKey dipakai ulang cache koreksi transkrip. Kosong bila kunci gagal
	// dihitung (berkas tidak terbaca), dan itu bukan galat: yang hilang cuma
	// cachenya.
	CacheKey string
	// WAV = audio hasil ekstraksi. Kosong bila transkrip datang dari cache dan
	// pemanggil tidak meminta audionya.
	WAV        string
	FromCache  bool
	ExtractDur time.Duration
	WhisperDur time.Duration
}

// Transcript menghasilkan transkrip satu video: cache → ekstrak audio → whisper.
//
// Dipakai bersama oleh pipeline klip dan pembuat caption, sebab keduanya
// membutuhkan tahap yang sama persis — termasuk cachenya, yang membuat mencoba
// ulang tidak berarti membayar tahap termahal untuk kedua kalinya.
//
// maxSec > 0 hanya mentranskripsi sekian detik pertama (lihat caption).
// wantAudio memaksa audio tetap diekstrak walau transkripnya sudah ada di
// cache — dibutuhkan mesin skor heuristik yang membaca energi audionya.
func (p *Pipeline) Transcript(ctx context.Context, input, tmpDir string, maxSec float64, wantAudio bool, onProgress ProgressFunc) (TranscriptResult, error) {
	var res TranscriptResult
	if err := p.transcriberReady(); err != nil {
		return res, err
	}

	if key, err := transcriptCacheKey(input, p.transcriberCacheModel(), p.Opts.Language, maxSec); err == nil {
		res.CacheKey = key
		if cached, ok := loadTranscriptCache(transcriptCachePath(p.Paths.DataDir, key)); ok {
			res.Transcript, res.FromCache = cached, true
			emit(onProgress, Progress{Stage: "transcribing", Value: 0.48,
				Message: "Transcript loaded from cache (no re-transcription needed)"})
		}
	}

	wav := filepath.Join(tmpDir, "audio.wav")
	if !res.FromCache || wantAudio {
		emit(onProgress, Progress{Stage: "extracting", Value: 0.05, Message: "Extracting audio"})
		t0 := time.Now()
		// Tanpa trek suara tidak ada yang bisa dikerjakan: baik klip maupun
		// caption berdiri di atas ucapan. Diperiksa di sini supaya pesannya
		// menyebut sebabnya, bukan nama berkas keluaran yang gagal dibuat.
		if ok, err := p.ff.HasAudio(ctx, input); err == nil && !ok {
			return res, fmt.Errorf(
				"this video has no sound track, so there is nothing to transcribe: Clipper works from what is said. Pick a video that has audio")
		}
		if err := p.ff.ExtractAudioWAV(ctx, input, wav, maxSec); err != nil {
			return res, err
		}
		res.WAV = wav
		res.ExtractDur = time.Since(t0)
	}

	if !res.FromCache {
		emit(onProgress, Progress{Stage: "transcribing", Value: 0.2, Message: "Transcribing (" + p.TranscriberName() + ")"})
		t0 := time.Now()
		progress := func(f float64) {
			// Petakan 0..1 transkripsi ke pita 0.20..0.48 dari total.
			emit(onProgress, Progress{
				Stage:   "transcribing",
				Value:   0.20 + 0.28*f,
				Message: fmt.Sprintf("Transcribing %.0f%%", f*100),
			})
		}
		var got types.Transcript
		var err error
		encode := func(ctx context.Context, start, dur float64, out string) error {
			return p.ff.EncodeAudioChunk(ctx, wav, start, dur, out)
		}
		if o := p.openAIAudio(); o != nil {
			o.Encode = encode
			var sec float64
			if sec, err = p.ff.Duration(ctx, wav); err == nil {
				got, err = o.Transcribe(ctx, sec, p.Opts.Language, tmpDir, progress)
			}
		} else if p.aiStudio() {
			as := transcribe.AIStudioFromEnv()
			as.Encode = encode
			var sec float64
			if sec, err = p.ff.Duration(ctx, wav); err == nil {
				got, err = as.Transcribe(ctx, sec, p.Opts.Language, tmpDir, progress)
			}
		} else {
			got, err = p.wh.Transcribe(ctx, wav, filepath.Join(tmpDir, "transcript"), p.Opts.Language, runtime.NumCPU(), progress)
		}
		if err != nil {
			return res, err
		}
		res.Transcript = got
		res.WhisperDur = time.Since(t0)
		if res.CacheKey != "" {
			// Gagal menyimpan cache tidak boleh menggagalkan job.
			_ = saveTranscriptCache(transcriptCachePath(p.Paths.DataDir, res.CacheKey), got)
		}
	}

	if len(res.Transcript.Segments) == 0 {
		return res, fmt.Errorf("the transcript is empty: check the audio/language")
	}
	// Dihentikan DI SINI: melewatkan ribuan segmen halusinasi ke tahap
	// berikutnya makan waktu sangat lama dan hasilnya tetap sampah. Diperiksa
	// juga pada cache hit — transkrip buruk yang terlanjur tersimpan tidak boleh
	// lolos hanya karena tidak dihitung ulang.
	if err := detectRepetitionLoop(res.Transcript); err != nil {
		return res, err
	}
	return res, nil
}
