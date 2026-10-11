package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gemgum/clipper/engine/internal/types"
)

// probeBytes: jumlah byte yang dibaca dari awal & akhir video untuk sidik jari.
// Membaca seluruh berkas 1–4 jam terlalu mahal; kombinasi ukuran + potongan
// awal + potongan akhir sudah sangat kecil peluang tabrakannya.
const probeBytes = 4 << 20 // 4 MB

// transcriptCacheKey membuat kunci cache dari isi video + model + bahasa.
// Model/bahasa ikut dikunci karena transkrip berbeda untuk tiap kombinasi.
//
// Awalan versi ("v2") dinaikkan setiap kali flag decoding whisper berubah:
// flag tidak ikut masuk kunci, jadi tanpa itu transkrip lama akan dipakai ulang
// dan perbaikan decoding seolah-olah tidak berpengaruh. v2 = pemakaian -mc 0
// dan -sns (anti-loop halusinasi); transkrip v1 dibuang karena bisa mengandung
// loop tersebut.
//
// maxSec > 0 berarti hanya sekian detik pertama video yang ditranskripsi, dan
// ikut masuk kunci — transkrip separuh TIDAK boleh dipakai ulang oleh job klip
// yang membutuhkan video utuh. Nol tidak mengubah kunci sama sekali, jadi cache
// yang sudah ada tetap terpakai.
func transcriptCacheKey(video, model, lang string, maxSec float64) (string, error) {
	f, err := os.Open(video)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}

	h := sha256.New()
	fmt.Fprintf(h, "v2|%d|%s|%s|", st.Size(), model, lang)
	if maxSec > 0 {
		fmt.Fprintf(h, "t%.0f|", maxSec)
	}

	headLen := int64(probeBytes)
	if st.Size() < headLen {
		headLen = st.Size()
	}
	head := make([]byte, headLen)
	if _, err := io.ReadFull(f, head); err != nil {
		return "", err
	}
	h.Write(head)

	if st.Size() > probeBytes {
		off := st.Size() - probeBytes
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return "", err
		}
		tail := make([]byte, probeBytes)
		if _, err := io.ReadFull(f, tail); err != nil {
			return "", err
		}
		h.Write(tail)
	}
	return hex.EncodeToString(h.Sum(nil))[:32], nil
}

// transcriptCachePath lokasi berkas cache untuk satu kunci.
func transcriptCachePath(dataDir, key string) string {
	return filepath.Join(dataDir, "cache", "transcripts", key+".json")
}

// correctedCachePath lokasi transkrip yang sudah dikoreksi.
//
// Kuncinya menyertakan nama mesin & versi prompt: mengganti model atau
// memperbaiki prompt harus menghasilkan koreksi baru, bukan memakai ulang hasil
// lama yang bentuknya sudah berbeda. Disimpan terpisah dari transkrip mentah
// supaya mematikan koreksi tidak memaksa transkripsi ulang — tahap termahal.
func correctedCachePath(dataDir, transcriptKey, engine, version string) string {
	h := sha256.Sum256([]byte(transcriptKey + "\x00" + engine + "\x00" + version))
	return filepath.Join(dataDir, "cache", "corrected", hex.EncodeToString(h[:16])+".json")
}

// loadTranscriptCache membaca transkrip tersimpan (ok=false bila belum ada).
func loadTranscriptCache(path string) (types.Transcript, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return types.Transcript{}, false
	}
	var tr types.Transcript
	if err := json.Unmarshal(raw, &tr); err != nil || len(tr.Segments) == 0 {
		return types.Transcript{}, false
	}
	return tr, true
}

// saveTranscriptCache menyimpan transkrip (gagal simpan tidak menggagalkan job).
func saveTranscriptCache(path string, tr types.Transcript) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(tr)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
