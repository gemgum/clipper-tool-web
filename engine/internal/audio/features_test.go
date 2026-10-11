package audio

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestReadHeader(t *testing.T) {
	f, err := os.Open("testdata/audio.wav")
	if err != nil {
		t.Skip("no sample WAV available")
	}
	defer f.Close()

	info, err := readHeader(f)
	if err != nil {
		t.Fatalf("readHeader: %v", err)
	}
	t.Logf("%+v", info)
}

// writeWAV membuat WAV PCM 16-bit mono untuk keperluan uji.
// Bila withList true, sebuah chunk "LIST" disisipkan sebelum "data" —
// meniru keluaran ffmpeg asli dan menjebak asumsi "header selalu 44 byte".
func writeWAV(t *testing.T, samples []int16, sampleRate int, withList bool) string {
	t.Helper()

	var body bytes.Buffer // segalanya sesudah penanda "WAVE"

	body.WriteString("fmt ")
	binary.Write(&body, binary.LittleEndian, uint32(16))
	binary.Write(&body, binary.LittleEndian, uint16(1)) // 1 = PCM
	binary.Write(&body, binary.LittleEndian, uint16(1)) // mono
	binary.Write(&body, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&body, binary.LittleEndian, uint32(sampleRate*2)) // byte rate
	binary.Write(&body, binary.LittleEndian, uint16(2))            // block align
	binary.Write(&body, binary.LittleEndian, uint16(16))           // bit per sampel

	if withList {
		var info bytes.Buffer
		info.WriteString("INFO")
		info.WriteString("ISFT")
		soft := "Lavf60.100\x00"
		binary.Write(&info, binary.LittleEndian, uint32(len(soft)))
		info.WriteString(soft)

		body.WriteString("LIST")
		binary.Write(&body, binary.LittleEndian, uint32(info.Len()))
		body.Write(info.Bytes())
		if info.Len()%2 == 1 {
			body.WriteByte(0) // chunk ganjil diikuti satu byte sisipan
		}
	}

	body.WriteString("data")
	binary.Write(&body, binary.LittleEndian, uint32(len(samples)*2))
	for _, s := range samples {
		binary.Write(&body, binary.LittleEndian, s)
	}

	var out bytes.Buffer
	out.WriteString("RIFF")
	binary.Write(&out, binary.LittleEndian, uint32(4+body.Len()))
	out.WriteString("WAVE")
	out.Write(body.Bytes())

	path := filepath.Join(t.TempDir(), "test.wav")
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Nada tetap setengah skala: RMS tiap jendela harus tepat 0,5.
func TestFeaturesConstantAmplitude(t *testing.T) {
	const sr = 16000
	samples := make([]int16, sr) // 1 detik
	for i := range samples {
		samples[i] = 16384 // 16384/32768 = 0,5
	}

	res, err := Features(writeWAV(t, samples, sr, true), 100)
	if err != nil {
		t.Fatal(err)
	}
	if res.HopMS != 100 {
		t.Errorf("HopMS = %d, want 100", res.HopMS)
	}
	if len(res.RMS) != 10 { // 1 detik / 100 ms
		t.Fatalf("got %d windows, want 10", len(res.RMS))
	}
	for i, v := range res.RMS {
		if math.Abs(v-0.5) > 1e-9 {
			t.Errorf("window %d = %v, want 0.5", i, v)
		}
	}
}

// Jendela terakhir yang belum penuh tetap harus ikut dihitung.
func TestFeaturesTrailingPartialWindow(t *testing.T) {
	const sr = 16000
	samples := make([]int16, 1650) // 1 jendela penuh (1600) + sisa 50
	for i := range samples {
		samples[i] = 16384
	}

	res, err := Features(writeWAV(t, samples, sr, false), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.RMS) != 2 {
		t.Fatalf("got %d windows, want 2 (1600 + 50 samples)", len(res.RMS))
	}
	// Sisa 50 sampel dibagi 50, bukan 1600 — nilainya tetap 0,5.
	if math.Abs(res.RMS[1]-0.5) > 1e-9 {
		t.Errorf("partial window = %v, want 0.5", res.RMS[1])
	}
}

// Chunk metadata sebelum "data" tidak boleh mengubah hasil sedikit pun.
func TestFeaturesIgnoresMetadataChunk(t *testing.T) {
	const sr = 16000
	samples := make([]int16, 4000)
	for i := range samples {
		samples[i] = int16(20000 * math.Sin(2*math.Pi*440*float64(i)/sr))
	}

	plain, err := Features(writeWAV(t, samples, sr, false), 100)
	if err != nil {
		t.Fatal(err)
	}
	withList, err := Features(writeWAV(t, samples, sr, true), 100)
	if err != nil {
		t.Fatal(err)
	}

	if len(plain.RMS) != len(withList.RMS) {
		t.Fatalf("window count differs: %d vs %d", len(plain.RMS), len(withList.RMS))
	}
	for i := range plain.RMS {
		if plain.RMS[i] != withList.RMS[i] {
			t.Fatalf("window %d differs: %v vs %v", i, plain.RMS[i], withList.RMS[i])
		}
	}
}
