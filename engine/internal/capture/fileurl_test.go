package capture

import (
	"runtime"
	"testing"
)

// Path Windows harus jadi "file:///C:/…" — tiga garis miring, tanpa backslash.
//
// Bentuk yang salah ("file://C:\Users\…") membuat "C:" terbaca sebagai nama
// host oleh Chrome. Ia menolaknya tanpa pesan yang berguna: kartu berita
// tinggal keluar kosong, dan penyebabnya tidak kelihatan di mana pun.
func TestFileURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{`C:\Users\PHP-02\AppData\Local\Temp\kartu.html`, "file:///C:/Users/PHP-02/AppData/Local/Temp/kartu.html"},
		{"/tmp/kartu.html", "file:///tmp/kartu.html"},
		{`C:\Users\Budi Santoso\kartu.html`, "file:///C:/Users/Budi%20Santoso/kartu.html"},
		{"/home/a b/kartu.html", "file:///home/a%20b/kartu.html"},
	}
	for _, c := range cases {
		if got := fileURL(c.in); got != c.want {
			t.Errorf("fileURL(%q)\n  dapat %q\n  mau   %q", c.in, got, c.want)
		}
	}
}

// Penerjemahan path WSL hanya berlaku bila engine memang berjalan di Linux.
// Di Windows asli, browser .exe adalah hal yang normal — memanggil `wslpath`
// di sana gagal dengan "executable file not found in %PATH%".
func TestWindowsBinaryIsNotWSLInteropOnWindows(t *testing.T) {
	c := New(`C:\Program Files\Google\Chrome\Application\chrome.exe`)
	if got := c.isWindowsBin(); got != (runtime.GOOS == "linux") {
		t.Errorf("isWindowsBin() = %v di %s", got, runtime.GOOS)
	}
}
