package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

type wavInfo struct {
	sampleRate int
	channels   int
	bits       int
	dataOffset int64
	dataLen    int64
}

type FeaturesResult struct {
	RMS   []float64
	HopMS int
}

func readHeader(f *os.File) (wavInfo, error) {
	var info wavInfo

	var riff [12]byte
	if _, err := f.ReadAt(riff[:], 0); err != nil {
		return info, fmt.Errorf("read RIFF header: %w", err)
	}
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return info, errors.New("not a RIFF/WAVE file")
	}
	pos := int64(12)
	for {
		var head [8]byte
		if _, err := f.ReadAt(head[:], pos); err != nil {
			break
		}
		id := string(head[0:4])
		size := int64(binary.LittleEndian.Uint32(head[4:8]))
		body := pos + 8

		switch id {
		case "fmt ":
			var b [16]byte
			if _, err := f.ReadAt(b[:], body); err != nil {
				return info, fmt.Errorf("truncated fmt chunk: %w", err)
			}
			info.channels = int(binary.LittleEndian.Uint16(b[2:4]))
			info.sampleRate = int(binary.LittleEndian.Uint32(b[4:8]))
			info.bits = int(binary.LittleEndian.Uint16(b[14:16]))
		case "data":
			info.dataOffset = body
			info.dataLen = size
		}
		pos = body + size + (size & 1)
	}

	if info.dataOffset == 0 {
		return info, errors.New("no data chunk found")
	}
	if info.bits != 16 {
		return info, fmt.Errorf("unsupported sample size: %d-bit (expected 16-bit PCM)", info.bits)
	}
	if info.channels < 1 || info.sampleRate <= 0 {
		return info, fmt.Errorf("invalid format: %d channels at %d Hz", info.channels, info.sampleRate)
	}

	st, err := f.Stat()
	if err != nil {
		return info, err
	}
	if info.dataOffset+info.dataLen > st.Size() {
		info.dataLen = st.Size() - info.dataOffset
	}
	return info, nil
}

func Features(path string, hopMS int) (FeaturesResult, error) {
	if hopMS <= 0 {
		hopMS = 100
	}

	f, err := os.Open(path)
	if err != nil {
		return FeaturesResult{}, err
	}
	defer f.Close()

	info, err := readHeader(f)
	if err != nil {
		return FeaturesResult{}, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	hop := info.sampleRate * hopMS / 1000
	if hop < 1 {
		hop = 1
	}

	frameSize := 2 * info.channels
	res := FeaturesResult{HopMS: hopMS}

	if frames := info.dataLen / int64(frameSize); frames > 0 {
		res.RMS = make([]float64, 0, frames/int64(hop)+1)
	}

	sec := io.NewSectionReader(f, info.dataOffset, info.dataLen)

	block := 64 * 1024
	block -= block % frameSize
	buf := make([]byte, block)

	var (
		sum float64
		n   int
	)

	for {
		got, err := io.ReadFull(sec, buf)

		for i := 0; i+1 < got; i += frameSize {
			s := int16(binary.LittleEndian.Uint16(buf[i : i+2]))
			v := float64(s) / 32768.0
			sum += v * v
			n++
			if n == hop {
				res.RMS = append(res.RMS, math.Sqrt(sum/float64(n)))
				sum, n = 0, 0
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return FeaturesResult{}, fmt.Errorf("read samples: %w", err)
		}
	}

	if n > 0 {
		res.RMS = append(res.RMS, math.Sqrt(sum/float64(n)))
	}
	return res, nil
}
