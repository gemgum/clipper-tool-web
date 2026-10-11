package news

import "testing"

// Feed TIDAK dijamin urut; ini kasus yang benar-benar terjadi — artikel
// unggulan di puncak, yang terbaru di tengah, satu tanpa tanggal.
func TestSortNewestFirst(t *testing.T) {
	in := []Article{
		{Title: "unggulan lama", Published: "2026-08-01T10:00:00Z"},
		{Title: "tanpa tanggal", Published: ""},
		{Title: "paling baru", Published: "2026-08-06T09:00:00Z"},
		{Title: "kemarin", Published: "2026-08-05T20:00:00Z"},
	}
	sortNewestFirst(in)
	want := []string{"paling baru", "kemarin", "unggulan lama", "tanpa tanggal"}
	for i, w := range want {
		if in[i].Title != w {
			t.Fatalf("urutan[%d] = %q, mau %q (hasil: %v)", i, in[i].Title, w, titles(in))
		}
	}
}

func titles(a []Article) []string {
	out := make([]string, len(a))
	for i, x := range a {
		out[i] = x.Title
	}
	return out
}
