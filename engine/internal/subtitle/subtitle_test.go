package subtitle

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/types"
)

// utterance membuat segmen dengan kata berdurasi tetap mulai detik start.
func utterance(start, per float64, words ...string) types.TranscriptSegment {
	seg := types.TranscriptSegment{Start: start, Text: strings.Join(words, " ")}
	t := start
	for _, w := range words {
		seg.Words = append(seg.Words, types.Word{Start: t, End: t + per, Text: w})
		t += per
	}
	seg.End = t
	return seg
}

func TestBuildPagesHasNoFlashPages(t *testing.T) {
	// Ucapan cepat (0,25 dtk/kata) — dulu menghasilkan tampilan <0,5 detik.
	segs := []types.TranscriptSegment{
		utterance(0, 0.25, "hari", "ini", "kita", "bahas", "soal", "uang", "dan", "cara", "mengaturnya", "dengan", "benar."),
		utterance(3.0, 0.25, "itu."),
	}
	sub := config.DefaultSubtitle()
	minDur, maxLines := sub.Pacing()
	pages := buildPages(collectWords(segs, 0), maxCharsPerLine(sub.Size), maxLines, minDur)

	if len(pages) == 0 {
		t.Fatal("tidak ada tampilan dihasilkan")
	}
	for i, p := range pages {
		if d := p.end - p.start; d < 0.9 {
			t.Errorf("tampilan %d hanya %.2f detik (terlalu cepat dibaca)", i, d)
		}
	}
	// Tampilan tidak boleh saling tumpang tindih.
	for i := 1; i < len(pages); i++ {
		if pages[i].start < pages[i-1].end {
			t.Errorf("tampilan %d mulai (%.2f) sebelum %d selesai (%.2f)",
				i, pages[i].start, i-1, pages[i-1].end)
		}
	}
}

func TestBuildPagesLosesNoWords(t *testing.T) {
	segs := []types.TranscriptSegment{
		utterance(0, 0.3, "satu", "dua", "tiga", "empat", "lima", "enam", "tujuh", "delapan", "sembilan", "sepuluh"),
	}
	pages := buildPages(collectWords(segs, 0), 20, 2, 1.2)
	var got []string
	for _, p := range pages {
		for _, ln := range p.lines {
			for _, w := range ln {
				got = append(got, w.Text)
			}
		}
	}
	want := "satu dua tiga empat lima enam tujuh delapan sembilan sepuluh"
	if strings.Join(got, " ") != want {
		t.Errorf("kata berubah/hilang:\n dapat: %s\n ingin: %s", strings.Join(got, " "), want)
	}
}

func TestTimingFollowsWordTimestamps(t *testing.T) {
	// Kata pertama mulai detik 5 dalam klip yang dimulai detik 3 → relatif 2 dtk.
	segs := []types.TranscriptSegment{utterance(5, 0.5, "halo", "dunia")}
	words := collectWords(segs, 3)
	if len(words) != 2 {
		t.Fatalf("dapat %d kata", len(words))
	}
	if words[0].Start < 1.99 || words[0].Start > 2.01 {
		t.Errorf("waktu mulai relatif salah: %.2f (ingin 2.00)", words[0].Start)
	}
}

func TestColorIsNotForcedToYellow(t *testing.T) {
	sub := config.DefaultSubtitle()
	sub.Color = "white"
	sub.Mode = config.SubKaraoke
	head := assHeader(sub, config.Headline{}, false)
	// PrimaryColour (kolom ke-4 baris Style) harus tetap putih.
	for _, line := range strings.Split(head, "\n") {
		if !strings.HasPrefix(line, "Style: Default,") {
			continue
		}
		cols := strings.Split(line, ",")
		if cols[3] != "&H00FFFFFF" {
			t.Errorf("warna dasar berubah jadi %s padahal pengguna memilih putih", cols[3])
		}
	}
	if hl := highlightColor(sub); hl != "&H0000FFFF" {
		t.Errorf("warna sorot = %s, ingin kuning", hl)
	}
}

func TestWordModeShowsOneWordPerPage(t *testing.T) {
	segs := []types.TranscriptSegment{utterance(0, 0.4, "satu", "dua", "tiga")}
	sub := config.DefaultSubtitle()
	sub.Mode = config.SubWord

	path := t.TempDir() + "/w.ass"
	if err := WriteASS(path, segs, 0, sub, config.Watermark{}, "", 0); err != nil {
		t.Fatal(err)
	}
	var dialogue []string
	for _, l := range strings.Split(readFile(t, path), "\n") {
		if strings.HasPrefix(l, "Dialogue:") {
			dialogue = append(dialogue, l)
		}
	}
	if len(dialogue) != 3 {
		t.Fatalf("dapat %d baris Dialogue, ingin 3 (satu per kata)", len(dialogue))
	}
	for i, want := range []string{"satu", "dua", "tiga"} {
		if !strings.HasSuffix(dialogue[i], want) {
			t.Errorf("baris %d: %q tidak berakhir dengan kata %q", i, dialogue[i], want)
		}
	}
}

func TestKaraokeModeHighlightsActiveWord(t *testing.T) {
	segs := []types.TranscriptSegment{utterance(0, 0.4, "satu", "dua", "tiga")}
	sub := config.DefaultSubtitle()
	sub.Mode = config.SubKaraoke

	path := t.TempDir() + "/k.ass"
	if err := WriteASS(path, segs, 0, sub, config.Watermark{}, "", 0); err != nil {
		t.Fatal(err)
	}
	body := readFile(t, path)
	// Tiap kata dapat gilirannya disorot, seluruh teks tetap terlihat.
	for _, w := range []string{"satu", "dua", "tiga"} {
		if !strings.Contains(body, "&}"+w+"{") {
			t.Errorf("kata %q tidak pernah disorot", w)
		}
	}
	n := strings.Count(body, "Dialogue:")
	if n != 3 {
		t.Errorf("dapat %d Dialogue, ingin 3 (satu per pergantian sorot)", n)
	}
}

// Pacing mengikuti nilai kecepatan bahasa Inggris; nilai lama bahasa Indonesia
// tidak lagi dikenali dan jatuh ke normal.
func TestPacingFollowsSpeed(t *testing.T) {
	sub := config.DefaultSubtitle()
	sub.Speed = config.SpeedSlow
	slowDur, slowLines := sub.Pacing()
	sub.Speed = config.SpeedDense
	denseDur, denseLines := sub.Pacing()
	if slowDur <= denseDur {
		t.Errorf("slow (%.1f) harus menahan tampilan lebih lama dari dense (%.1f)", slowDur, denseDur)
	}
	if slowLines >= denseLines {
		t.Errorf("slow (%d baris) harus lebih sedikit dari dense (%d baris)", slowLines, denseLines)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Titik yang dipilih pengguna di preview harus berarti "di sini baris pertama
// mulai". Dengan jangkar tengah (an5) halaman 2 baris mengangkang titik itu:
// baris pertama naik dan posisinya berubah-ubah mengikuti jumlah baris.
func TestAnchorIsTopSoFirstLineNeverMoves(t *testing.T) {
	// Satu ucapan pendek (1 baris) lalu satu ucapan panjang (pasti >1 baris).
	segs := []types.TranscriptSegment{
		utterance(0, 0.5, "halo."),
		utterance(2, 0.4, "kalimat", "ini", "sengaja", "dibuat", "sangat",
			"panjang", "supaya", "terpaksa", "dipecah", "menjadi", "dua", "baris."),
	}
	sub := config.DefaultSubtitle()
	sub.X, sub.Y = 540, 1500

	path := t.TempDir() + "/a.ass"
	if err := WriteASS(path, segs, 0, sub, config.Watermark{}, "", 0); err != nil {
		t.Fatal(err)
	}

	var oneLine, multiLine int
	for _, l := range strings.Split(readFile(t, path), "\n") {
		if !strings.HasPrefix(l, "Dialogue:") {
			continue
		}
		if strings.Contains(l, `\an5`) {
			t.Fatalf("jangkar masih di tengah blok (an5): %s", l)
		}
		if !strings.Contains(l, `{\an8\pos(540,1500)}`) {
			t.Errorf("jangkar tidak di tepi atas titik pengguna: %s", l)
		}
		if strings.Contains(l, `\N`) {
			multiLine++
		} else {
			oneLine++
		}
	}
	// Tanpa keduanya, tes ini tidak membuktikan apa pun soal jumlah baris.
	if oneLine == 0 || multiLine == 0 {
		t.Fatalf("perlu halaman 1 baris dan >1 baris; dapat %d dan %d", oneLine, multiLine)
	}
}

// Berkas .txt dibaca MESIN LAIN untuk membuat caption. Timestamp, nomor baris,
// dan escape khas .ass di sana bukan cuma tidak berguna — ia memakan token dan
// bisa ditafsirkan model sebagai bagian dari isi.
func TestPlainTextCarriesNoTimingArtefacts(t *testing.T) {
	segs := []types.TranscriptSegment{
		utterance(0, 0.5, "Halo", "semua."),
		utterance(2, 0.4, "Ini", "kalimat", "kedua", "yang", "sengaja", "dibuat",
			"panjang", "supaya", "terpaksa", "dipecah", "jadi", "beberapa", "baris."),
		utterance(9, 0.4, "Dan", "ini", "yang", "terakhir!"),
	}
	path := t.TempDir() + "/a.txt"
	if err := WriteText(path, segs, 0); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)

	for _, bad := range []string{"-->", `\N`, "{", "}", "00:00"} {
		if strings.Contains(got, bad) {
			t.Errorf("teks masih memuat %q:\n%s", bad, got)
		}
	}
	// Tidak ada baris yang cuma berisi angka (penomoran gaya .srt).
	for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
		if line == "" {
			t.Error("ada baris kosong — .txt ini bukan .srt")
		}
		if _, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
			t.Errorf("baris %q cuma angka, terlihat seperti penomoran .srt", line)
		}
	}
	// Dan isinya memang ucapan yang lengkap, urut.
	for _, want := range []string{"Halo semua.", "Dan ini yang terakhir!"} {
		if !strings.Contains(got, want) {
			t.Errorf("ucapan %q hilang dari:\n%s", want, got)
		}
	}
	if i, j := strings.Index(got, "Halo"), strings.Index(got, "terakhir"); i > j {
		t.Error("urutan ucapan terbalik")
	}
}

// Satu kalimat satu baris — supaya berkasnya juga enak dibaca manusia saat
// diperiksa, bukan satu gumpalan tanpa jeda.
func TestPlainTextBreaksAtSentenceEnds(t *testing.T) {
	segs := []types.TranscriptSegment{
		utterance(0, 0.5, "Kalimat", "satu.", "Kalimat", "dua.", "Kalimat", "tiga."),
	}
	path := t.TempDir() + "/a.txt"
	if err := WriteText(path, segs, 0); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(readFile(t, path)), "\n")
	if len(lines) != 3 {
		t.Errorf("dapat %d baris, mau 3:\n%q", len(lines), lines)
	}
}

// Klip tanpa ucapan tetap menghasilkan berkas. Kalau tidak, pengguna harus
// menebak apakah berkasnya gagal ditulis atau memang tidak ada yang diucapkan.
func TestPlainTextIsStillWrittenWhenNobodySpeaks(t *testing.T) {
	path := t.TempDir() + "/a.txt"
	if err := WriteText(path, nil, 0); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "" {
		t.Errorf("mau kosong, dapat %q", got)
	}
}

// Kata di luar rentang klip tidak boleh ikut terbawa — caption harus tentang
// klipnya, bukan tentang bagian video yang tidak dipotong.
func TestPlainTextKeepsOnlyWhatIsInsideTheClip(t *testing.T) {
	segs := []types.TranscriptSegment{
		utterance(0, 0.5, "sebelum", "klip."),
		utterance(10, 0.5, "di", "dalam", "klip."),
	}
	path := t.TempDir() + "/a.txt"
	if err := WriteText(path, segs, 10); err != nil { // klip mulai di detik 10
		t.Fatal(err)
	}
	got := readFile(t, path)
	if strings.Contains(got, "sebelum") {
		t.Errorf("ucapan di luar klip ikut terbawa:\n%s", got)
	}
	if !strings.Contains(got, "di dalam klip.") {
		t.Errorf("ucapan di dalam klip hilang:\n%s", got)
	}
}

// Tidak ada baris yang diawali spasi menggantung. Sepele di layar, tapi berkas
// ini dibaca mesin — spasi di awal baris ikut jadi token tanpa makna.
func TestPlainTextHasNoHangingSpaces(t *testing.T) {
	segs := []types.TranscriptSegment{
		utterance(0, 0.5, "Satu.", "Dua.", "Tiga."),
	}
	path := t.TempDir() + "/a.txt"
	if err := WriteText(path, segs, 0); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(readFile(t, path), "\n") {
		if line != strings.TrimSpace(line) {
			t.Errorf("baris %q punya spasi di tepi", line)
		}
	}
}

// "Sorot kata": kata aktif mendapat blok (garis tepi tebal berwarna sorotan,
// teks gelap), dan gaya dasar dipulihkan sesudahnya dengan {\r} — kata lain
// tidak ikut tertutup blok.
func TestHighlightModeDrawsBlockBehindActiveWord(t *testing.T) {
	segs := []types.TranscriptSegment{utterance(0, 0.4, "satu", "dua", "tiga")}
	sub := config.DefaultSubtitle()
	sub.Mode = config.SubHighlight
	sub.Size = 94

	path := t.TempDir() + "/h.ass"
	if err := WriteASS(path, segs, 0, sub, config.Watermark{}, "", 0); err != nil {
		t.Fatal(err)
	}
	body := readFile(t, path)
	block := `{\1c&H001D1815&\3c&H0000FFFF&\bord18\shad0}`
	for _, w := range []string{"satu", "dua", "tiga"} {
		if !strings.Contains(body, block+w+`{\r}`) {
			t.Errorf("kata %q tidak pernah mendapat blok sorotan", w)
		}
	}
	if n := strings.Count(body, "Dialogue:"); n != 3 {
		t.Errorf("dapat %d Dialogue, ingin 3", n)
	}
	if n := strings.Count(body, `\bord18`); n != 3 {
		t.Errorf("blok muncul %d kali, ingin tepat 1 per Dialogue", n)
	}
}
