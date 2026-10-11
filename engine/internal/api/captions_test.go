package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// post membuat permintaan JSON ke endpoint caption.
func postCaption(body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/captions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	(&Server{}).createCaption(w, r)
	return w
}

// Alamat video datang dari klien (berkas lokal TIDAK diunggah, notes/24), jadi
// ia diperiksa SEBELUM job dibuat: kalau tidak, kekeliruan paling umum — salah
// ketik, berkas dipindah — baru terlihat sebagai galat ffmpeg belasan detik
// kemudian, di kotak log, tanpa menyebut berkas mana.
func TestCreateCaptionChecksInput(t *testing.T) {
	cases := map[string]struct {
		body string
		want int
	}{
		"no videos":    {`{"videos":[]}`, 400},
		"missing file": {`{"videos":["/tidak/ada/video.mp4"]}`, 400},
		"a folder":     {`{"videos":["/tmp"]}`, 400},
		"too many":     {`{"videos":[` + strings.Repeat(`"/tmp/a.mp4",`, maxCaptionVideos) + `"/tmp/b.mp4"]}`, 400},
		"not JSON":     {`videos=1`, 400},
	}
	for name, c := range cases {
		if got := postCaption(c.body).Code; got != c.want {
			t.Errorf("%s: status = %d, mau %d", name, got, c.want)
		}
	}
}

// Job yang belum selesai tidak punya berkas, dan berkasnya ditunjuk lewat nomor
// urut — nama yang datang dari luar adalah jalan keluar dari folder hasil.
func TestCaptionFileNeedsFinishedJob(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest("GET", "/api/captions/caption_0001/file?i=0", nil)
	r.SetPathValue("id", "caption_0001")
	w := httptest.NewRecorder()
	s.captionFile(w, r)
	if w.Code != 404 {
		t.Errorf("status = %d, mau 404", w.Code)
	}
}
