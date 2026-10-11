package writer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// articleHTML membuat halaman berita tiruan: tag og: lengkap dan paragraf yang
// cukup panjang untuk lolos ambang news.minParagraphWords.
func articleHTML(canonical, title, site string, paras ...string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, `<html><head>
		<meta property="og:title" content=%q>
		<meta property="og:url" content=%q>
		<meta property="og:site_name" content=%q>
		<meta property="og:image" content="https://gambar.example/foto.jpg">
	</head><body>`, title, canonical, site)
	for _, p := range paras {
		fmt.Fprintf(&sb, "<p>%s</p>", p)
	}
	sb.WriteString("</body></html>")
	return sb.String()
}

const (
	paraGempa1 = "Gempa berkekuatan magnitudo 5,2 mengguncang wilayah Kabupaten Bantul pada Senin pukul 03.14 WIB menurut BMKG."
	paraGempa2 = "Pusat gempa berada di laut pada kedalaman 24 kilometer dan dipastikan tidak berpotensi tsunami sama sekali."
	paraGempa3 = "Badan Penanggulangan Bencana Daerah Bantul melaporkan tujuh rumah rusak ringan dan tidak ada korban jiwa."
	paraBola   = "Timnas Indonesia menang tiga gol tanpa balas atas Vietnam pada laga kualifikasi yang digelar di Jakarta."
	paraBola2  = "Pelatih menyebut kemenangan itu hasil kerja keras seluruh pemain selama masa pemusatan latihan di Bali."
)

// newsServer menyajikan beberapa halaman berita tiruan.
func newsServer(t *testing.T, pages map[string]func(base string) string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, body(srv.URL))
	}))
	return srv
}

// TestGatherDedupSetelahResolve menjaga jebakan notes/38: artikel yang sama
// gampang masuk dua kali lewat alamat yang berbeda, dan dedup harus terjadi
// SESUDAH alamat kanoniknya diketahui.
func TestGatherDedupSetelahResolve(t *testing.T) {
	srv := newsServer(t, map[string]func(string) string{
		"/asli": func(base string) string {
			return articleHTML(base+"/asli", "Gempa guncang Bantul", "Contoh", paraGempa1, paraGempa2)
		},
		// Alamat berbeda, og:url sama — persis kasus tautan yang dibagikan.
		"/ikutan": func(base string) string {
			return articleHTML(base+"/asli", "Gempa guncang Bantul", "Contoh", paraGempa1, paraGempa2)
		},
	})
	defer srv.Close()

	b, err := Gather(context.Background(), []string{srv.URL + "/asli", srv.URL + "/ikutan?utm=wa"}, nil, t.TempDir(), "id")
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(b.Sources) != 1 {
		t.Errorf("sumber = %d, mau 1 (artikel yang sama masuk dua kali)", len(b.Sources))
	}
	if len(b.Skipped) != 1 {
		t.Fatalf("yang dilewati = %d, mau 1: %+v", len(b.Skipped), b.Skipped)
	}
	if !strings.Contains(b.Skipped[0].Reason, "same article") {
		t.Errorf("alasan = %q", b.Skipped[0].Reason)
	}
}

// TestGatherBatasLima: batasnya di keranjang, bukan per jalan masuk.
func TestGatherBatasLima(t *testing.T) {
	pages := map[string]func(string) string{}
	var urls []string
	for i := 0; i < MaxSources+2; i++ {
		p := fmt.Sprintf("/a%d", i)
		i := i
		pages[p] = func(base string) string {
			return articleHTML(fmt.Sprintf("%s/a%d", base, i), fmt.Sprintf("Gempa Bantul bagian %d", i), "Contoh", paraGempa1, paraGempa2)
		}
	}
	srv := newsServer(t, pages)
	defer srv.Close()
	for i := 0; i < MaxSources+2; i++ {
		urls = append(urls, fmt.Sprintf("%s/a%d", srv.URL, i))
	}

	b, err := Gather(context.Background(), urls, nil, t.TempDir(), "id")
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(b.Sources) != MaxSources {
		t.Errorf("sumber = %d, mau %d", len(b.Sources), MaxSources)
	}
	if len(b.Skipped) != 2 {
		t.Errorf("yang dilewati = %d, mau 2", len(b.Skipped))
	}
}

// TestGatherMelaporkanYangGagal: artikel yang hilang tanpa kabar terbaca
// sebagai "sumbernya memang cuma segitu".
func TestGatherMelaporkanYangGagal(t *testing.T) {
	srv := newsServer(t, map[string]func(string) string{
		"/ada": func(base string) string {
			return articleHTML(base+"/ada", "Gempa guncang Bantul", "Contoh", paraGempa1, paraGempa2)
		},
	})
	defer srv.Close()

	b, err := Gather(context.Background(), []string{srv.URL + "/ada", srv.URL + "/hilang"}, nil, t.TempDir(), "id")
	if err != nil {
		t.Fatalf("satu artikel gagal tidak boleh menggagalkan keranjang: %v", err)
	}
	if len(b.Sources) != 1 || len(b.Skipped) != 1 {
		t.Fatalf("sumber=%d dilewati=%d", len(b.Sources), len(b.Skipped))
	}
	if b.Skipped[0].Reason == "" {
		t.Error("alasan gagal harus ikut")
	}
}

// TestGatherGagalBilaKosong: nol sumber berarti tidak ada yang bisa ditulis,
// dan pesannya harus menyebutkan sebab tiap alamat ditolak.
func TestGatherGagalBilaKosong(t *testing.T) {
	srv := newsServer(t, nil)
	defer srv.Close()
	_, err := Gather(context.Background(), []string{srv.URL + "/hilang"}, nil, t.TempDir(), "id")
	if err == nil {
		t.Fatal("mau galat karena tidak ada sumber yang terpakai")
	}
	if !strings.Contains(err.Error(), "/hilang") {
		t.Errorf("pesan galat tidak menyebut alamat yang bermasalah: %v", err)
	}
}

// TestGatherMenandaiTopikJauh: DITANDAI, bukan ditolak (notes/38).
func TestGatherMenandaiTopikJauh(t *testing.T) {
	srv := newsServer(t, map[string]func(string) string{
		"/gempa": func(base string) string {
			return articleHTML(base+"/gempa", "Gempa magnitudo 5,2 guncang Bantul", "Contoh", paraGempa1, paraGempa2)
		},
		"/gempa2": func(base string) string {
			return articleHTML(base+"/gempa2", "Gempa Bantul, BPBD catat kerusakan", "Contoh Dua", paraGempa2, paraGempa3)
		},
		"/bola": func(base string) string {
			return articleHTML(base+"/bola", "Timnas Indonesia kalahkan Vietnam", "Contoh Tiga", paraBola, paraBola2)
		},
	})
	defer srv.Close()

	b, err := Gather(context.Background(), []string{srv.URL + "/gempa", srv.URL + "/gempa2", srv.URL + "/bola"}, nil, t.TempDir(), "id")
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(b.Sources) != 3 {
		t.Fatalf("topik jauh tidak boleh ditolak, sumber = %d", len(b.Sources))
	}
	if len(b.OffTopic) != 1 || !strings.Contains(b.OffTopic[0], "Timnas") {
		t.Errorf("OffTopic = %+v, mau menandai artikel bola", b.OffTopic)
	}
}
