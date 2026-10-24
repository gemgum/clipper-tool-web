// Package fontmetrics mengukur lebar teks dari berkas font yang SAMA dengan
// yang dipakai libass, bukan dari tebakan lebar huruf rata-rata.
//
// Dipakai kotak judul berita (subtitle.NewsBox): tebakan rata-rata meleset
// pada teks huruf kecil yang panjang dan teksnya keluar dari kotak (notes/44).
// Yang dibaca cuma tabel head, OS/2, hhea, hmtx, dan cmap format 4 — cukup
// untuk huruf Latin; ditulis tangan karena proyek ini memakai pustaka standar
// saja. Kerning diabaikan, jadi hasilnya sedikit LEBIH lebar dari render:
// arah aman untuk menjaga teks di dalam kotak.
package fontmetrics

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Face = lebar maju tiap huruf satu font, dalam satuan Fontsize .ass.
type Face struct {
	box  float64 // winAscent + winDescent: satu "Fontsize" .ass (lihat api/fontmetrics.go)
	adv  []uint16
	cmap map[rune]uint16
}

// Width = lebar s dalam satuan Fontsize: kalikan dengan ukuran huruf untuk
// mendapat piksel ruang PlayRes.
func (f *Face) Width(s string) float64 {
	w := 0
	for _, r := range s {
		g := int(f.cmap[r]) // huruf yang tidak ada = glyph 0 (.notdef)
		if g >= len(f.adv) {
			g = len(f.adv) - 1
		}
		w += int(f.adv[g])
	}
	return float64(w) / f.box
}

var cache sync.Map // path → *Face

// ForFamily mencari berkas font sebuah family di dir, persis cara libass
// memilih face lewat fontsdir: "<Family>-Bold.ttf" untuk teks tebal, lalu
// "<Family>-Regular.ttf", lalu "<Family>.ttf". nil = tidak ketemu.
func ForFamily(dir, family string, bold bool) *Face {
	if dir == "" || family == "" {
		return nil
	}
	names := []string{family + "-Regular.ttf", family + ".ttf"}
	if bold {
		names = append([]string{family + "-Bold.ttf"}, names...)
	}
	for _, n := range names {
		if f, err := Load(filepath.Join(dir, n)); err == nil {
			return f
		}
	}
	return nil
}

// Load membaca satu berkas .ttf/.otf (bukan koleksi .ttc).
func Load(path string) (*Face, error) {
	if f, ok := cache.Load(path); ok {
		return f.(*Face), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	cache.Store(path, f)
	return f, nil
}

func parse(raw []byte) (*Face, error) {
	u16 := func(o int) int { return int(binary.BigEndian.Uint16(raw[o:])) }
	if len(raw) < 12 {
		return nil, fmt.Errorf("font file is too short")
	}
	n := u16(4)
	tables := map[string]int{}
	for i := 0; i < n && 12+16*i+16 <= len(raw); i++ {
		e := 12 + 16*i
		tables[string(raw[e:e+4])] = int(binary.BigEndian.Uint32(raw[e+8:]))
	}
	need := func(tag string, size int) (int, error) {
		o, ok := tables[tag]
		if !ok || o+size > len(raw) {
			return 0, fmt.Errorf("font has no usable %s table", tag)
		}
		return o, nil
	}
	os2, err := need("OS/2", 78)
	if err != nil {
		return nil, err
	}
	hhea, err := need("hhea", 36)
	if err != nil {
		return nil, err
	}
	hmtx, err := need("hmtx", 0)
	if err != nil {
		return nil, err
	}
	cmapAt, err := need("cmap", 4)
	if err != nil {
		return nil, err
	}

	f := &Face{box: float64(u16(os2+74) + u16(os2+76)), cmap: map[rune]uint16{}}
	if f.box == 0 {
		return nil, fmt.Errorf("font has no line box")
	}
	nh := u16(hhea + 34)
	if nh == 0 || hmtx+4*nh > len(raw) {
		return nil, fmt.Errorf("font hmtx table is out of range")
	}
	for i := 0; i < nh; i++ {
		f.adv = append(f.adv, uint16(u16(hmtx+4*i)))
	}

	// cmap format 4 dari subtabel Unicode BMP (platform 3/1 atau 0/x).
	sub := -1
	for i := 0; i < u16(cmapAt+2) && cmapAt+4+8*i+8 <= len(raw); i++ {
		e := cmapAt + 4 + 8*i
		plat, enc := u16(e), u16(e+2)
		o := cmapAt + int(binary.BigEndian.Uint32(raw[e+4:]))
		if (plat == 3 && enc == 1 || plat == 0) && o+14 <= len(raw) && u16(o) == 4 {
			sub = o
			break
		}
	}
	if sub < 0 {
		return nil, fmt.Errorf("font has no Unicode cmap (format 4)")
	}
	segs := u16(sub+6) / 2
	ends, starts := sub+14, sub+16+2*segs
	deltas, ranges := starts+2*segs, starts+4*segs
	if ranges+2*segs > len(raw) {
		return nil, fmt.Errorf("font cmap is out of range")
	}
	for i := 0; i < segs; i++ {
		end, start := u16(ends+2*i), u16(starts+2*i)
		delta, ro := u16(deltas+2*i), u16(ranges+2*i)
		for c := start; c <= end && c != 0xFFFF; c++ {
			g := 0
			if ro == 0 {
				g = (c + delta) & 0xFFFF
			} else {
				at := ranges + 2*i + ro + 2*(c-start)
				if at+2 > len(raw) {
					continue
				}
				if g = u16(at); g != 0 {
					g = (g + delta) & 0xFFFF
				}
			}
			if g != 0 {
				f.cmap[rune(c)] = uint16(g)
			}
		}
	}
	return f, nil
}
