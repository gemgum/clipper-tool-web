// Package subtitle menghasilkan berkas subtitle (.ass) untuk klip.
package subtitle

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/types"
)

const (
	playResX = 1080
	playResY = 1920
	margin   = 60 // margin kiri/kanan (ruang aman)

	gapBreak   = 0.8 // jeda diam (detik) yang memulai tampilan baru
	maxPageDur = 7.0 // satu tampilan tidak lebih lama dari ini
	holdMax    = 1.0 // perpanjangan maksimum ke jeda sesudahnya
	wordHold   = 0.4 // perpanjangan maksimum tiap kata di mode "word"
)

func assColor(c string) string {
	switch strings.ToLower(c) {
	case "yellow":
		return "&H0000FFFF"
	case "white", "":
		return "&H00FFFFFF"
	case "black":
		return "&H00000000"
	}
	if strings.HasPrefix(c, "#") && len(c) == 7 {
		r, g, b := c[1:3], c[3:5], c[5:7]
		return "&H00" + strings.ToUpper(b+g+r)
	}
	return "&H00FFFFFF"
}

// styleColors: warna pilihan pengguna SELALU jadi warna dasar. Sorot karaoke
// ditempel per kata lewat tag \c inline, bukan dengan menukar warna gaya —
// dulu warna dipaksa kuning begitu karaoke aktif.
func styleColors(s config.Subtitle) (primary, secondary string) {
	base := assColor(s.Color)
	return base, base
}

// highlightColor warna sorot kata aktif; jatuh ke kuning (atau putih bila dasar
// sudah kuning) supaya sorotan selalu kelihatan.
func highlightColor(s config.Subtitle) string {
	hl := s.HighlightColor
	if hl == "" || strings.EqualFold(hl, s.Color) {
		hl = "yellow"
		if strings.EqualFold(s.Color, "yellow") {
			hl = "white"
		}
	}
	return assColor(hl)
}

func assHeader(s config.Subtitle, hl config.Headline, withHeadline bool) string {
	bold := 0
	if s.Bold {
		bold = 1
	}
	font := s.Font
	if font == "" {
		font = "Montserrat"
	}
	primary, secondary := styleColors(s)
	outline := s.Outline
	if outline < 0 {
		outline = 4
	}
	borderStyle, shadow := 1, 2
	back := "&H90000000"
	if s.Box {
		borderStyle = 3
		shadow = 0
		back = "&HA0000000"
	}
	// WrapStyle 2 = hanya baris manual (\N); kita atur pembungkusan sendiri.
	return fmt.Sprintf(`[Script Info]
ScriptType: v4.00+
PlayResX: %d
PlayResY: %d
WrapStyle: 2

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,%s,%d,%s,%s,%s,%s,%d,0,0,0,100,100,0,0,%d,%d,%d,5,%d,%d,40,1
%s
[Events]
`, playResX, playResY, font, s.Size, primary, secondary, assColor(s.OutlineColor), back, bold, borderStyle, outline, shadow, margin, margin, headlineStyle(hl, withHeadline)) +
		"Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"
}

// headlineStyle menambah SATU style lagi untuk teks watermark.
//
// Style kedua, bukan tag inline di tiap baris: font, ukuran, warna, dan tebal
// garis tepinya berdiri sendiri dari subtitle — headline itu identitas akun,
// subtitle itu ucapan. Style-nya hanya ditulis bila memang ada yang memakainya;
// .ass berisi style tak terpakai tetap sah, tapi berkas yang ditulis untuk
// dibaca manusia saat menelusuri kegagalan sebaiknya tidak mengarang isi.
func headlineStyle(h config.Headline, on bool) string {
	if !on {
		return ""
	}
	bold := 0
	if h.Bold {
		bold = 1
	}
	// BorderStyle 1 (garis tepi + bayangan), bukan 3 (kotak): teks ini sering
	// berdiri di atas gambar videonya sendiri, dan garis tepi cukup membuatnya
	// terbaca tanpa menutupi apa pun.
	return fmt.Sprintf("\nStyle: Headline,%s,%d,%s,%s,%s,&H90000000,%d,0,0,0,100,100,0,0,1,%d,0,5,%d,%d,40,1",
		h.Font, h.Size, assColor(h.Color), assColor(h.Color), assColor(h.OutlineColor), bold, h.Outline, margin, margin)
}

// headlinePad = ruang napas di dalam kotak, di kedua sisi.
//
// Kecil dan TETAP, bukan margin bingkai 60: kotaknya sendiri bisa cuma
// seperempat lebar layar, dan margin sebesar itu memakan separuhnya.
const headlinePad = 16

// headlineLines memenggal teks headline supaya muat di dalam KOTAKNYA.
//
// boxW = lebar kotak dalam satuan PlayRes (bukan persen). Kotaknya adalah
// gambar watermark bila ada, atau seluruh bingkai bila tidak — lihat
// config.Watermark.HeadlineBox.
//
// Acuan kotak, bukan bingkai. Sempat bingkai, dan itu keliru: teks yang ditulis
// untuk duduk di dalam kartu meluber keluar kartunya, dan tidak ada satu pun
// batas yang menahannya.
//
// Baris yang diketik pengguna sendiri dihormati lebih dulu: kalau ia menekan
// enter, di situlah ia ingin barisnya patah.
func headlineLines(text string, size, boxW int) []string {
	if size <= 0 {
		size = 64
	}
	usable := boxW - 2*headlinePad
	maxChars := int(float64(usable) / (float64(size) * 0.6))
	// Di bawah 4 huruf teksnya tidak lagi terbaca sebagai kata. Kotak sesempit
	// itu berarti fontnya yang harus dikecilkan, dan pratinjau menunjukkannya
	// meluber — jujur, dan bisa diperbaiki pengguna.
	if maxChars < 4 {
		maxChars = 4
	}

	var out []string
	for _, para := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			switch {
			case line == "":
				line = word
			case len(line)+1+len(word) <= maxChars:
				line += " " + word
			default:
				out = append(out, line)
				line = word
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// writeHeadline menulis teks watermark sebagai baris statis.
//
// Layer 1: ia harus berada DI ATAS subtitle bila keduanya kebetulan bertumpang
// tindih — subtitle datang dan pergi, headline adalah identitas yang tidak boleh
// tertutup.
func writeHeadline(b *strings.Builder, watermark config.Watermark, text string, clipDur float64) {
	// Sisi GUI (wrapHeadline & clampHeadline di gui/app/watermark-model.ts)
	// menghitung dengan aturan yang sama persis; kalau tidak, pratinjau
	// memenggal dan mengurung di tempat lain daripada hasil rendernya.
	cx, cy, boxW, boxH := watermark.HeadlineBox()
	lines := headlineLines(escapeText(text), watermark.Headline.Size, boxW)
	if len(lines) == 0 {
		return
	}
	x, y := headlineAnchor(lines, watermark.Headline, cx, cy, boxW, boxH)
	end := watermark.At + watermark.For
	if watermark.For <= 0 {
		// Sampai klip habis. Klip tanpa durasi yang diketahui (pemanggil lama,
		// atau .ass yang ditulis di luar render) diberi ujung yang jauh: .ass
		// yang berakhir setelah videonya sudah selesai tidak apa-apa, sedangkan
		// yang berakhir terlalu awal membuat watermark hilang di tengah jalan.
		end = clipDur
		if end <= 0 {
			end = 3600
		}
	}
	// an5 = jangkar di TENGAH blok teks, bukan tepi atasnya (an8 yang dipakai
	// subtitle). Bedanya berarti di sini: headline dikurung sebuah kotak, dan
	// "di tengah kotak" hanya bisa tepat kalau titik jangkarnya juga tengah.
	// Subtitle memakai an8 karena jumlah barisnya berubah sepanjang klip; blok
	// headline tidak pernah berubah, jadi alasan itu tidak berlaku.
	fmt.Fprintf(b, "Dialogue: 1,%s,%s,Headline,,0,0,0,,{\\an5\\pos(%d,%d)}%s\n",
		tc(watermark.At), tc(end), x, y, strings.Join(lines, "\\N"))
}

// headlineAnchor menaruh blok teks di dalam kotaknya: tengah kotak, digeser
// sebanyak DX/DY, lalu DIJEPIT supaya seluruh bloknya tetap di dalam.
//
// Penjepitannya memakai perkiraan yang sama dengan pemenggalan (0,6 x ukuran
// per huruf, satu baris setinggi ukurannya) — bukan pengukuran font sungguhan.
// Itu cukup: yang dijaga di sini bukan presisi sub-piksel melainkan janji bahwa
// teksnya tidak keluar dari gambar yang memuatnya.
func headlineAnchor(lines []string, h config.Headline, cx, cy, boxW, boxH int) (int, int) {
	longest := 0
	for _, ln := range lines {
		if len(ln) > longest {
			longest = len(ln)
		}
	}
	blockW := int(float64(longest) * float64(h.Size) * 0.6)
	blockH := len(lines) * h.Size

	// Blok yang lebih besar daripada kotaknya tidak bisa dijepit ke dalam —
	// yang bisa dilakukan cuma menengahkannya, dan membiarkan pratinjau
	// menunjukkan bahwa fontnya terlalu besar.
	limitX := max(0, (boxW-blockW)/2)
	limitY := max(0, (boxH-blockH)/2)
	return cx + clamp(h.DX, -limitX, limitX), cy + clamp(h.DY, -limitY, limitY)
}

func clamp(v, lo, hi int) int { return min(hi, max(lo, v)) }

func tc(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	h := int(sec) / 3600
	m := (int(sec) % 3600) / 60
	s := int(sec) % 60
	cs := int((sec - float64(int(sec))) * 100)
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}

func escapeText(t string) string {
	t = strings.ReplaceAll(t, "\n", " ")
	t = strings.ReplaceAll(t, "{", "(")
	t = strings.ReplaceAll(t, "}", ")")
	return strings.TrimSpace(t)
}

// maxCharsPerLine memperkirakan jumlah karakter yang muat 1 baris berdasarkan
// ukuran font & lebar aman. Faktor 0.6 (over-estimate) agar tidak meluber.
func maxCharsPerLine(size int) int {
	if size <= 0 {
		size = 72
	}
	usable := playResX - 2*margin
	n := int(float64(usable) / (float64(size) * 0.6))
	if n < 6 {
		n = 6
	}
	return n
}

// page satu tampilan subtitle: beberapa baris kata dengan rentang waktunya.
type page struct {
	lines [][]types.Word
	start float64
	end   float64
}

func (p page) words() int {
	n := 0
	for _, ln := range p.lines {
		n += len(ln)
	}
	return n
}

// collectWords menggabungkan kata dari semua segmen dalam klip menjadi satu
// aliran waktu relatif terhadap awal klip. Menyatukan aliran (bukan per segmen)
// mencegah potongan kilat seperti satu kata "itu." tampil 0,4 detik sendirian.
func collectWords(segs []types.TranscriptSegment, clipStart float64) []types.Word {
	var out []types.Word
	for _, s := range segs {
		for _, w := range s.WordList() {
			text := escapeText(w.Text)
			if text == "" {
				continue
			}
			start := w.Start - clipStart
			end := w.End - clipStart
			if end <= 0 {
				continue
			}
			if start < 0 {
				start = 0
			}
			if end <= start {
				end = start + 0.12
			}
			out = append(out, types.Word{Start: start, End: end, Text: text})
		}
	}
	return out
}

// buildPages memecah aliran kata menjadi tampilan. Batas tampilan: jumlah baris,
// lebar baris, jeda diam, dan durasi maksimum. Tampilan yang lebih pendek dari
// minDur diperpanjang ke jeda sesudahnya bila ruangnya ada.
func buildPages(words []types.Word, maxChars, maxLines int, minDur float64) []page {
	var pages []page
	var cur [][]types.Word // baris-baris tampilan berjalan
	var line []types.Word
	lineLen := 0

	flush := func() {
		if len(line) > 0 {
			cur = append(cur, line)
			line, lineLen = nil, 0
		}
		if len(cur) == 0 {
			return
		}
		p := page{lines: cur}
		p.start = cur[0][0].Start
		lastLine := cur[len(cur)-1]
		p.end = lastLine[len(lastLine)-1].End
		pages = append(pages, p)
		cur = nil
	}

	for i, w := range words {
		// Jeda diam panjang atau kalimat berakhir & sudah cukup lama → tampilan baru.
		if len(cur) > 0 || len(line) > 0 {
			var pageStart float64
			if len(cur) > 0 {
				pageStart = cur[0][0].Start
			} else {
				pageStart = line[0].Start
			}
			prevEnd := w.Start
			if i > 0 {
				prevEnd = words[i-1].End
			}
			gap := w.Start - prevEnd
			dur := prevEnd - pageStart
			if gap >= gapBreak || dur >= maxPageDur ||
				(dur >= minDur && i > 0 && endsSentence(words[i-1].Text)) {
				flush()
			}
		}

		add := len(w.Text)
		if lineLen > 0 {
			add++ // spasi
		}
		if lineLen > 0 && lineLen+add > maxChars {
			cur = append(cur, line)
			line, lineLen = nil, 0
			if len(cur) >= maxLines {
				flush()
			}
			add = len(w.Text)
		}
		line = append(line, w)
		lineLen += add
	}
	flush()

	// Perpanjang tampilan pendek ke jeda sesudahnya (tanpa menabrak berikutnya).
	for i := range pages {
		if pages[i].end-pages[i].start >= minDur {
			continue
		}
		limit := pages[i].end + holdMax
		if i+1 < len(pages) {
			if pages[i+1].start < limit {
				limit = pages[i+1].start
			}
		}
		want := pages[i].start + minDur
		if want < limit {
			limit = want
		}
		if limit > pages[i].end {
			pages[i].end = limit
		}
	}
	return pages
}

func endsSentence(t string) bool {
	t = strings.TrimRight(strings.TrimSpace(t), `"')`)
	if t == "" {
		return false
	}
	last := t[len(t)-1]
	return last == '.' || last == '?' || last == '!'
}

// WriteASS menulis .ass sesuai mode: normal (kalimat utuh), karaoke (kata aktif
// disorot), atau word (satu kata per layar).
// watermark & clipDur menambahkan lapis watermark: headline ditulis sebagai baris
// statis berstyle sendiri di berkas yang SAMA. Banner PNG-nya bukan urusan sini
// — itu overlay ffmpeg (internal/ffmpeg) — tapi lebarnya ikut dibaca, sebab ia
// yang menentukan di mana teks headline boleh patah.
//
// headlineText dipisah dari watermark.Headline.Text supaya pemanggil bisa
// menyerahkan judul PER KLIP (sumber "llm") tanpa menyalin seluruh struct.
func WriteASS(path string, segs []types.TranscriptSegment, clipStart float64, sub config.Subtitle, watermark config.Watermark, headlineText string, clipDur float64) error {
	x, y := sub.X, sub.Y
	if x <= 0 {
		x = playResX / 2
	}
	if y <= 0 {
		y = playResY / 2
	}
	// an8 = jangkar di tepi ATAS blok teks, bukan di tengahnya (an5).
	// Dengan an5 sebuah halaman 2 baris mengangkang titik jangkar: baris pertama
	// naik setengah baris dari tempat yang dipilih pengguna di preview, dan
	// tinggi blok ikut berubah tiap kali jumlah barisnya berubah — subtitle
	// terlihat melompat-lompat sepanjang klip. Dengan an8 titik itu berarti
	// "di sini baris pertama mulai": baris tambahan turun ke bawah mengikuti
	// arah baca, dan baris pertama tidak pernah bergeser.
	pos := fmt.Sprintf("{\\an8\\pos(%d,%d)}", x, y)

	words := collectWords(segs, clipStart)
	var b strings.Builder
	headlineText = strings.TrimSpace(headlineText)
	b.WriteString(assHeader(sub, watermark.Headline, headlineText != ""))
	writeHeadline(&b, watermark, headlineText, clipDur)

	if sub.Mode == config.SubWord {
		writeWordMode(&b, words, pos)
		return os.WriteFile(path, []byte(b.String()), 0o644)
	}

	minDur, maxLines := sub.Pacing()
	pages := buildPages(words, maxCharsPerLine(sub.Size), maxLines, minDur)
	for _, p := range pages {
		if sub.Mode == config.SubKaraoke {
			writeKaraokePage(&b, p, pos, karaokeTag(highlightColor(sub)))
			continue
		}
		if sub.Mode == config.SubHighlight {
			writeKaraokePage(&b, p, pos, blockTag(highlightColor(sub), sub.Size))
			continue
		}
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s%s\n", tc(p.start), tc(p.end), pos, renderPlain(p))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// WriteText menulis ucapan satu klip sebagai teks polos, tanpa timestamp dan
// tanpa nomor.
//
// Bukan pengganti .srt: .srt untuk editor video, berkas ini untuk DIBACA MESIN
// LAIN — ditempel ke LLM mana pun untuk dibuatkan caption. Timestamp dan nomor
// baris justru mengganggu di sana; keduanya memakan token dan tidak menambah
// makna apa pun bagi model yang cuma perlu tahu apa yang diucapkan.
//
// Ditulis untuk SETIAP klip, apa pun mode simpannya. Berbeda dari .srt yang
// hanya ada di mode clean/both — caption dibutuhkan saat memposting, dan orang
// memposting klip bersubtitle juga.
//
// Pemenggalan barisnya mengikuti akhir kalimat, bukan lebar layar: berkas ini
// tidak pernah tampil di video, jadi lebar baris subtitle tidak relevan.
// Satu kalimat satu baris juga membuatnya enak dibaca manusia saat diperiksa.
func WriteText(path string, segs []types.TranscriptSegment, clipStart float64) error {
	out := Text(segs, clipStart)
	if out == "" {
		// Klip tanpa ucapan sama sekali (musik, ambience). Berkasnya tetap
		// ditulis kosong supaya jumlah berkas per klip selalu sama — pengguna
		// tidak perlu menebak apakah berkasnya hilang atau memang tak ada isi.
		return os.WriteFile(path, nil, 0o644)
	}
	return os.WriteFile(path, []byte(out+"\n"), 0o644)
}

// Text menyusun isi berkas itu tanpa menuliskannya.
//
// Dipakai pembuat caption, yang butuh ucapan klip sebagai teks untuk dikirim ke
// LLM — bentuk yang sama persis dengan yang selama ini ditempel orang dari
// berkas .txt, jadi tidak ada dua gagasan berbeda tentang "ucapan tanpa waktu".
func Text(segs []types.TranscriptSegment, clipStart float64) string {
	var b strings.Builder
	atLineStart := true
	for _, w := range collectWords(segs, clipStart) {
		// escapeText di collectWords menyisipkan escape khas .ass; di teks polos
		// itu justru sampah, jadi dikembalikan.
		text := strings.ReplaceAll(w.Text, `\N`, " ")
		// Spasi hanya ANTAR kata dalam satu baris. Menulisnya tanpa syarat
		// membuat tiap baris setelah yang pertama diawali spasi menggantung.
		if !atLineStart {
			b.WriteByte(' ')
		}
		b.WriteString(text)
		atLineStart = endsSentence(text)
		if atLineStart {
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}

// WriteSRT menulis subtitle .srt (untuk dipakai di editor lain saat pengguna
// memilih menyimpan klip polos). Isinya mengikuti pemenggalan yang sama.
func WriteSRT(path string, segs []types.TranscriptSegment, clipStart float64, sub config.Subtitle) error {
	minDur, maxLines := sub.Pacing()
	pages := buildPages(collectWords(segs, clipStart), maxCharsPerLine(sub.Size), maxLines, minDur)

	var b strings.Builder
	for i, p := range pages {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n",
			i+1, srtTime(p.start), srtTime(p.end),
			strings.ReplaceAll(renderPlain(p), `\N`, "\n"))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func srtTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	h := int(sec) / 3600
	m := (int(sec) % 3600) / 60
	s := int(sec) % 60
	ms := int(math.Round((sec - float64(int(sec))) * 1000))
	if ms > 999 {
		ms = 999
	}
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

// renderPlain menggabungkan baris dengan \N.
func renderPlain(p page) string {
	rendered := make([]string, len(p.lines))
	for i, ln := range p.lines {
		parts := make([]string, len(ln))
		for j, w := range ln {
			parts[j] = w.Text
		}
		rendered[i] = strings.Join(parts, " ")
	}
	return strings.Join(rendered, `\N`)
}

// karaokeTag: kata aktif berganti warna saja.
func karaokeTag(hl string) string { return fmt.Sprintf("{\\c%s&}", hl) }

// blockTag: kata aktif berdiri di atas BLOK warna sorotan ("Sorot kata",
// DESIGN.md §4.2). .ass tidak punya latar per kata, jadi bloknya adalah garis
// tepi yang sangat tebal (\bord ≈ ukuran/5) berwarna sorotan, tanpa bayangan,
// dengan teks gelap di atasnya — garis tepi tiap huruf menyatu jadi satu
// bidang. Tidak perlu mengukur lebar huruf, jadi bloknya selalu pas dengan
// katanya di font apa pun.
func blockTag(hl string, size int) string {
	bord := size / 5
	if bord < 6 {
		bord = 6
	}
	return fmt.Sprintf("{\\1c&H001D1815&\\3c%s&\\bord%d\\shad0}", hl, bord)
}

// writeKaraokePage menulis satu Dialogue per kata: seluruh tampilan tetap
// terlihat dalam gaya dasar, hanya kata yang sedang diucapkan yang diberi tag
// `on`; sesudahnya {\r} mengembalikan gaya dasar. Waktu ganti sorot memakai
// timestamp kata asli.
func writeKaraokePage(b *strings.Builder, p page, pos, on string) {
	flat := make([]types.Word, 0, p.words())
	for _, ln := range p.lines {
		flat = append(flat, ln...)
	}
	for i := range flat {
		start := flat[i].Start
		if i == 0 {
			start = p.start
		}
		end := p.end
		if i+1 < len(flat) {
			end = flat[i+1].Start
		}
		if end <= start {
			continue
		}
		fmt.Fprintf(b, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s%s\n",
			tc(start), tc(end), pos, renderHighlighted(p, i, on))
	}
}

// renderHighlighted membangun teks tampilan dengan kata ke-active disorot.
func renderHighlighted(p page, active int, on string) string {
	idx := 0
	rendered := make([]string, len(p.lines))
	for i, ln := range p.lines {
		parts := make([]string, len(ln))
		for j, w := range ln {
			if idx == active {
				parts[j] = on + w.Text + "{\\r}"
			} else {
				parts[j] = w.Text
			}
			idx++
		}
		rendered[i] = strings.Join(parts, " ")
	}
	return strings.Join(rendered, `\N`)
}

// writeWordMode menulis satu Dialogue per kata (satu kata per layar). Tiap kata
// bertahan sampai kata berikutnya muncul (maks wordHold) agar tidak berkedip.
func writeWordMode(b *strings.Builder, words []types.Word, pos string) {
	for i, w := range words {
		end := w.End + wordHold
		if i+1 < len(words) && words[i+1].Start < end {
			end = words[i+1].Start
		}
		if end <= w.Start {
			end = w.Start + 0.12
		}
		fmt.Fprintf(b, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s%s\n", tc(w.Start), tc(end), pos, w.Text)
	}
}
