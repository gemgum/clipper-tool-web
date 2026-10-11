package writer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// Post = berkas yang ditulis ke disk untuk satu artikel.
type Post struct {
	Dir      string `json:"dir"`
	Markdown string `json:"markdown"`
	Sources  string `json:"sources"`
	Image    string `json:"image,omitempty"`
	// ImageNote menerangkan kenapa gambarnya tidak ada, bila memang tidak ada.
	ImageNote string `json:"image_note,omitempty"`
}

// phrases = teks tetap yang ditulis ENGINE ke berkas pendamping. Mengikuti
// parameter lang dari klien, seperti paket card; bawaan "en".
type phrases struct {
	attribution string
	unverified  string
	checkFirst  string
}

var phraseBook = map[string]phrases{
	"en": {
		attribution: "Summarised from: ",
		unverified:  "Unverified",
		checkFirst:  "check these against the sources before publishing",
	},
	"id": {
		attribution: "Dirangkum dari: ",
		unverified:  "Belum terverifikasi",
		checkFirst:  "periksa terhadap sumbernya sebelum terbit",
	},
}

func phrasesFor(lang string) phrases {
	if p, ok := phraseBook[strings.ToLower(strings.TrimSpace(lang))]; ok {
		return p
	}
	return phraseBook["en"]
}

// Save menulis satu artikel beserta pendampingnya ke folder sendiri.
//
//	<base>/2026-08-17_11-04-judul-artikel/
//	  artikel.md    judul + badan, dengan blok peringatan di atas bila ada
//	  sumber.json   artikel sumber + fakta + peta klaim + pelanggaran
//	  gambar.jpg    og:image dari artikel sumber (bila ada)
//
// Gambar yang gagal diunduh TIDAK menggagalkan penyimpanan: artikelnya jauh
// lebih berharga daripada gambarnya, dan alasannya dicatat di ImageNote.
func Save(ctx context.Context, base string, draft Draft, sources []Source, lang string, now time.Time) (Post, error) {
	dir, err := makeDir(base, draft.Title, now)
	if err != nil {
		return Post{}, err
	}
	post := Post{Dir: dir}

	post.Markdown = filepath.Join(dir, "artikel.md")
	if err := os.WriteFile(post.Markdown, []byte(markdown(draft, sources, lang)), 0o644); err != nil {
		return Post{}, err
	}

	post.Sources = filepath.Join(dir, "sumber.json")
	b, err := json.MarshalIndent(sourcesFile(draft, sources, lang, now), "", "  ")
	if err != nil {
		return Post{}, err
	}
	if err := os.WriteFile(post.Sources, b, 0o644); err != nil {
		return Post{}, err
	}

	if path, err := saveImage(ctx, dir, sources); err != nil {
		post.ImageNote = err.Error()
	} else {
		post.Image = path
	}
	return post, nil
}

// markdown menyusun artikel.md.
//
// Blok peringatan sengaja ditulis DI DALAM berkasnya, bukan cuma ditampilkan di
// GUI: keputusan notes/38 adalah pagar tidak pernah menggagalkan job, dan itu
// hanya aman kalau peringatannya ikut terbawa saat berkasnya disalin. Pemilik
// boleh menghapusnya — tapi harus sengaja, bukan karena tidak pernah tahu.
func markdown(d Draft, sources []Source, lang string) string {
	p := phrasesFor(lang)
	var sb strings.Builder

	if len(d.Violations) > 0 {
		fmt.Fprintf(&sb, "> **%s (%d)**: %s:\n", p.unverified, len(d.Violations), p.checkFirst)
		for _, v := range d.Violations {
			fmt.Fprintf(&sb, "> - `%s` %q: %s\n", v.Kind, v.Text, v.Detail)
		}
		sb.WriteString(">\n\n")
	}

	fmt.Fprintf(&sb, "# %s\n\n", d.Title)
	if d.Lead != "" {
		fmt.Fprintf(&sb, "%s\n\n", d.Lead)
	}
	for _, para := range d.Body {
		fmt.Fprintf(&sb, "%s\n\n", para)
	}

	if len(d.Tags) > 0 {
		fmt.Fprintf(&sb, "%s\n\n", strings.Join(d.Tags, " "))
	}

	sb.WriteString("---\n\n")
	sb.WriteString(sourceLinks(sources, lang))
	return sb.String()
}

// sourceLinks menulis kaki artikel: nama media DAN alamat tiap artikel sumber.
//
// Alamatnya ikut karena berkas ini yang ditempel ke media milik pemilik proyek,
// dan menyebut "Antara News" tanpa tautan bukan atribusi — pembaca tidak bisa
// memeriksa, dan media sumbernya tidak mendapat apa pun. Ditulis ENGINE, bukan
// model: baris yang wajib ada tidak boleh bergantung pada model yang mungkin
// lupa (notes/38).
func sourceLinks(sources []Source, lang string) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSuffix(phrasesFor(lang).attribution, " ") + "\n\n")
	seen := map[string]bool{}
	for _, s := range sources {
		u := strings.TrimSpace(s.Facts.URL)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		title := strings.TrimSpace(s.Article.Article.Title)
		if title == "" {
			title = u
		}
		media := strings.TrimSpace(s.Facts.Source)
		fmt.Fprintf(&sb, "- [%s](%s)", title, u)
		if media != "" {
			fmt.Fprintf(&sb, ", %s", media)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// attribution ditempel ENGINE, bukan model — baris yang wajib ada tidak boleh
// bergantung pada model yang mungkin lupa (notes/38).
func attribution(sources []Source, lang string) string {
	seen := map[string]bool{}
	var names []string
	for _, s := range sources {
		n := s.Facts.Source
		if n == "" {
			n = s.Facts.URL
		}
		if n != "" && !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	return phrasesFor(lang).attribution + strings.Join(names, ", ")
}

// sumberFile = bentuk sumber.json.
//
// Ini alat kerja redaktur, bukan metadata: saat memeriksa sebelum terbit, tiap
// klaim bisa dilacak ke artikel dan paragrafnya tanpa membaca ulang semua
// sumber. Lembar fakta ikut karena ia jejak audit yang selalu ada, sementara
// "claims" bergantung pada model (notes/38).
type sumberFile struct {
	GeneratedAt string        `json:"generated_at"`
	Title       string        `json:"title"`
	Attribution string        `json:"attribution"`
	Words       int           `json:"words"`
	Tags        []string      `json:"tags,omitempty"`
	Repaired    bool          `json:"repaired,omitempty"`
	Violations  []Violation   `json:"violations,omitempty"`
	Claims      []Claim       `json:"claims,omitempty"`
	Sources     []sourceEntry `json:"sources"`
}

type sourceEntry struct {
	Index int        `json:"index"`
	Title string     `json:"title"`
	Media string     `json:"media"`
	URL   string     `json:"url"`
	Image string     `json:"image,omitempty"`
	Facts []Fact     `json:"facts"`
	Drop  []Rejected `json:"rejected_facts,omitempty"`
}

func sourcesFile(d Draft, sources []Source, lang string, now time.Time) sumberFile {
	f := sumberFile{
		GeneratedAt: now.Format(time.RFC3339),
		Title:       d.Title,
		Attribution: attribution(sources, lang),
		Words:       d.Words,
		Tags:        d.Tags,
		Repaired:    d.Repaired,
		Violations:  d.Violations,
		Claims:      d.Claims,
	}
	for i, s := range sources {
		f.Sources = append(f.Sources, sourceEntry{
			Index: i,
			Title: s.Article.Article.Title,
			Media: s.Facts.Source,
			URL:   s.Facts.URL,
			Image: s.Article.Article.Image,
			Facts: s.Facts.Facts,
			Drop:  s.Facts.Rejected,
		})
	}
	return f
}

// maxImageBytes membatasi unduhan gambar. Foto berita jauh di bawah ini; yang
// dijaga adalah alamat yang ternyata menunjuk berkas raksasa.
const maxImageBytes = 12 << 20

// saveImage mengunduh og:image sumber PERTAMA yang punya gambar.
//
// Sumber pertama dipilih karena itulah artikel yang dipilih pengguna lebih
// dulu; tidak ada penilaian mana foto yang "lebih bagus" — itu keputusan
// redaksi, bukan keputusan kode.
func saveImage(ctx context.Context, dir string, sources []Source) (string, error) {
	for _, s := range sources {
		raw := strings.TrimSpace(s.Article.Article.Image)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}
		path, err := download(ctx, raw, dir)
		if err != nil {
			return "", err
		}
		return path, nil
	}
	return "", fmt.Errorf("no source article carries an image")
}

func download(ctx context.Context, src, dir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return "", err
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("image download returned HTTP %d", resp.StatusCode)
	}

	name := "gambar" + imageExt(resp.Header.Get("Content-Type"), src)
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, io.LimitReader(resp.Body, maxImageBytes)); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// imageExt menentukan akhiran berkas dari Content-Type, dan jatuh ke akhiran di
// alamatnya bila header-nya tidak menolong.
func imageExt(contentType, src string) string {
	switch {
	case strings.Contains(contentType, "png"):
		return ".png"
	case strings.Contains(contentType, "webp"):
		return ".webp"
	case strings.Contains(contentType, "jpeg"), strings.Contains(contentType, "jpg"):
		return ".jpg"
	}
	if u, err := url.Parse(src); err == nil {
		if ext := strings.ToLower(filepath.Ext(u.Path)); ext == ".png" || ext == ".webp" || ext == ".jpg" || ext == ".jpeg" {
			if ext == ".jpeg" {
				return ".jpg"
			}
			return ext
		}
	}
	return ".jpg"
}

// makeDir membuat folder artikel, menambah akhiran angka bila sudah ada.
func makeDir(base, title string, now time.Time) (string, error) {
	name := now.Format("2006-01-02_15-04")
	if s := slug(title); s != "" {
		name += "_" + s
	}
	dir := filepath.Join(base, name)
	for i := 2; ; i++ {
		err := os.Mkdir(dir, 0o755)
		if err == nil {
			return dir, nil
		}
		if !os.IsExist(err) {
			// Folder induk mungkin belum ada pada pemakaian pertama.
			if err := os.MkdirAll(base, 0o755); err != nil {
				return "", err
			}
			if err := os.Mkdir(dir, 0o755); err == nil {
				return dir, nil
			} else if !os.IsExist(err) {
				return "", err
			}
		}
		if i > 99 {
			return "", fmt.Errorf("too many folders named %q", name)
		}
		dir = filepath.Join(base, fmt.Sprintf("%s-%d", name, i))
	}
}

// maxSlug menjaga panjang nama folder. Windows membatasi panjang path, dan
// judul berita Indonesia gampang melewati 100 karakter.
const maxSlug = 60

func slug(title string) string {
	var sb strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) && r < unicode.MaxASCII, unicode.IsDigit(r):
			sb.WriteRune(r)
			dash = false
		default:
			if !dash && sb.Len() > 0 {
				sb.WriteRune('-')
				dash = true
			}
		}
		if sb.Len() >= maxSlug {
			break
		}
	}
	return strings.Trim(sb.String(), "-")
}
