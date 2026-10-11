package writer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var stamp = time.Date(2026, 8, 17, 11, 4, 0, 0, time.UTC)

func draft() Draft {
	return Draft{
		Title: "Gempa magnitudo 5,2 guncang Bantul",
		Lead:  "Gempa mengguncang Bantul pada Senin dini hari.",
		Body:  []string{"Tujuh rumah rusak ringan di Kecamatan Kretek.", "Tidak ada korban jiwa."},
		Words: 12,
	}
}

// TestMarkdownMembawaPeringatanDalamBerkasnya menjaga syarat yang membuat
// kebijakan "pagar tidak menggagalkan job" aman: peringatannya harus ikut
// terbawa saat berkasnya disalin, bukan cuma tampil di GUI (notes/38).
func TestMarkdownMembawaPeringatanDalamBerkasnya(t *testing.T) {
	d := draft()
	d.Violations = []Violation{{"number", "19", "number does not appear in any source article"}}

	md := markdown(d, sources(), "id")
	if !strings.HasPrefix(md, ">") {
		t.Errorf("peringatan harus di paling atas berkas, dapat:\n%s", md)
	}
	for _, want := range []string{"Belum terverifikasi", "number", "19"} {
		if !strings.Contains(md, want) {
			t.Errorf("peringatan tidak memuat %q:\n%s", want, md)
		}
	}
	// Artikel bersih tidak boleh diberi blok peringatan.
	if md := markdown(draft(), sources(), "id"); strings.HasPrefix(md, ">") {
		t.Errorf("artikel bersih ikut diberi peringatan:\n%s", md)
	}
}

func TestMarkdownBentuknya(t *testing.T) {
	md := markdown(draft(), sources(), "id")
	if !strings.Contains(md, "# Gempa magnitudo 5,2 guncang Bantul") {
		t.Errorf("judul bukan H1:\n%s", md)
	}
	if !strings.Contains(md, "Dirangkum dari:") {
		t.Errorf("atribusi tidak ditempel:\n%s", md)
	}
	// Kaki artikel menyebut ALAMAT tiap sumber, bukan cuma nama medianya: berkas
	// ini yang ditempel ke media pemilik proyek, dan atribusi tanpa tautan tidak
	// bisa diperiksa pembaca.
	for _, s := range sources() {
		if !strings.Contains(md, "("+s.Facts.URL+")") {
			t.Errorf("tautan sumber %q hilang:\n%s", s.Facts.URL, md)
		}
		if !strings.Contains(md, s.Facts.Source) {
			t.Errorf("nama media %q hilang:\n%s", s.Facts.Source, md)
		}
	}
	for _, p := range draft().Body {
		if !strings.Contains(md, p) {
			t.Errorf("paragraf hilang: %q", p)
		}
	}
}

// TestAttributionDitempelEngine: baris wajib tidak boleh bergantung pada model,
// dan bahasanya mengikuti lang seperti berkas pendamping kartu.
func TestAttributionDitempelEngine(t *testing.T) {
	if got := attribution(sources(), "id"); got != "Dirangkum dari: Contoh, Contoh Dua" {
		t.Errorf("id: %q", got)
	}
	if got := attribution(sources(), "en"); got != "Summarised from: Contoh, Contoh Dua" {
		t.Errorf("en: %q", got)
	}
	// Bahasa yang tidak dikenal jatuh ke en, bukan kosong.
	if got := attribution(sources(), "jv"); !strings.HasPrefix(got, "Summarised from:") {
		t.Errorf("bahasa tak dikenal: %q", got)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Gempa magnitudo 5,2 guncang Bantul": "gempa-magnitudo-5-2-guncang-bantul",
		"  Trump: \"AS & Iran\"  ":           "trump-as-iran",
		"":                                   "",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, mau %q", in, got, want)
		}
	}
	// Judul panjang tidak boleh melahirkan nama folder raksasa.
	long := strings.Repeat("berita panjang ", 20)
	if got := slug(long); len(got) > maxSlug {
		t.Errorf("slug panjangnya %d, batas %d", len(got), maxSlug)
	}
}

func TestSaveMenulisKetiganya(t *testing.T) {
	srv := imageServer(t)
	defer srv.Close()
	srcs := sources()
	srcs[0].Article.Article.Image = srv.URL + "/foto.jpg"

	post, err := Save(context.Background(), t.TempDir(), draft(), srcs, "id", stamp)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if base := filepath.Base(post.Dir); !strings.HasPrefix(base, "2026-08-17_11-04_gempa") {
		t.Errorf("nama folder = %q", base)
	}
	for _, p := range []string{post.Markdown, post.Sources, post.Image} {
		if p == "" {
			t.Fatalf("berkas tidak ditulis: %+v", post)
		}
		if _, err := os.Stat(p); err != nil {
			t.Errorf("stat %s: %v", p, err)
		}
	}
	if filepath.Base(post.Image) != "gambar.jpg" {
		t.Errorf("nama gambar = %q", filepath.Base(post.Image))
	}

	// sumber.json harus memuat jejak audit yang bisa dipakai redaktur.
	b, err := os.ReadFile(post.Sources)
	if err != nil {
		t.Fatal(err)
	}
	var got sumberFile
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("sumber.json bukan JSON sah: %v", err)
	}
	if len(got.Sources) != 2 {
		t.Errorf("sumber = %d, mau 2", len(got.Sources))
	}
	if len(got.Sources[0].Facts) == 0 {
		t.Error("lembar fakta tidak ikut — itu jejak audit yang selalu ada")
	}
	if got.Sources[0].URL == "" {
		t.Error("tautan sumber hilang")
	}
}

// TestSaveTetapJadiTanpaGambar: artikelnya jauh lebih berharga daripada
// gambarnya, jadi gambar yang gagal tidak boleh menggagalkan penyimpanan.
func TestSaveTetapJadiTanpaGambar(t *testing.T) {
	srcs := sources()
	srcs[0].Article.Article.Image = "https://127.0.0.1:1/tidak-ada.jpg"

	post, err := Save(context.Background(), t.TempDir(), draft(), srcs, "id", stamp)
	if err != nil {
		t.Fatalf("Save gagal cuma karena gambar: %v", err)
	}
	if post.Image != "" {
		t.Errorf("Image = %q, mau kosong", post.Image)
	}
	if post.ImageNote == "" {
		t.Error("alasan gambar tidak ada harus dicatat, bukan hilang diam-diam")
	}
	if _, err := os.Stat(post.Markdown); err != nil {
		t.Errorf("artikel.md harus tetap ada: %v", err)
	}
}

// TestSaveFolderBentrok: dua artikel dalam menit yang sama tidak boleh saling
// menimpa.
func TestSaveFolderBentrok(t *testing.T) {
	base := t.TempDir()
	srcs := sources()
	first, err := Save(context.Background(), base, draft(), srcs, "id", stamp)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Save(context.Background(), base, draft(), srcs, "id", stamp)
	if err != nil {
		t.Fatal(err)
	}
	if first.Dir == second.Dir {
		t.Fatalf("folder kedua menimpa yang pertama: %s", first.Dir)
	}
}

func TestImageExt(t *testing.T) {
	cases := []struct{ ct, url, want string }{
		{"image/png", "https://x/a", ".png"},
		{"image/webp", "https://x/a", ".webp"},
		{"", "https://x/a.png", ".png"},
		{"", "https://x/a.jpeg?w=1", ".jpg"},
		{"text/html", "https://x/a", ".jpg"},
	}
	for _, c := range cases {
		if got := imageExt(c.ct, c.url); got != c.want {
			t.Errorf("imageExt(%q,%q) = %q, mau %q", c.ct, c.url, got, c.want)
		}
	}
}

func imageServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("\xff\xd8\xff\xe0 fake jpeg"))
	}))
}

// TestTagarDisaringTerhadapArtikel menjaga satu-satunya celah mengarang yang
// tersisa: tagar terlalu pendek untuk ditangkap pagar angka/kutipan/nama, tapi
// cukup untuk menempelkan artikel ke peristiwa yang tidak ada di dalamnya.
func TestTagarDisaringTerhadapArtikel(t *testing.T) {
	reply := `{"title":"Gempa Bantul","lead":"Gempa mengguncang Bantul.",
		"body":["Tujuh rumah rusak ringan di Kecamatan Kretek."],
		"tags":["Bantul","#Kretek","Tsunami Aceh",""]}`
	d, err := Compose(context.Background(), sources(), fakeCompleter(reply), "uji")
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	got := strings.Join(d.Tags, " ")
	for _, want := range []string{"#Bantul", "#Kretek"} {
		if !strings.Contains(got, want) {
			t.Errorf("tagar %q hilang dari %q", want, got)
		}
	}
	// "Tsunami Aceh" tidak ada di artikelnya — tidak boleh lolos.
	if strings.Contains(got, "Tsunami") {
		t.Errorf("tagar karangan lolos: %q", got)
	}

	// Dan ia ikut ke artikel.md, sebab di situlah ia ditempel orang.
	if md := markdown(d, sources(), "id"); !strings.Contains(md, "#Bantul") {
		t.Errorf("tagar tidak ditulis ke artikel.md:\n%s", md)
	}
}
