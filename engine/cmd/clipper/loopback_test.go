package main

import "testing"

// Mode web tanpa sandi hanya boleh di alamat yang tak terjangkau dari luar.
func TestLoopback(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:8787": true, "localhost:8787": true, "[::1]:8787": true, "127.0.0.1:0": true,
		"0.0.0.0:8787": false, ":8787": false, "192.168.1.5:8787": false, "[::]:8787": false, "example.com:80": false,
	} {
		if got := loopback(addr); got != want {
			t.Errorf("loopback(%q) = %v, mau %v", addr, got, want)
		}
	}
}

// Pengaturan VPS (127.0.0.1 di belakang nginx, -host publik) BUKAN lokal:
// tanpa ini situs publik terbuka tanpa sandi.
func TestLocalOnly(t *testing.T) {
	cases := []struct {
		addr, hosts string
		want        bool
	}{
		{"127.0.0.1:8787", "localhost", true},
		{"127.0.0.1:8787", "", true},
		{"127.0.0.1:8787", "localhost,127.0.0.1", true},
		{"127.0.0.1:8787", "klip.sarthlutions.id", false},
		{"127.0.0.1:8787", "localhost,klip.sarthlutions.id", false},
		{"0.0.0.0:8787", "localhost", false},
	}
	for _, c := range cases {
		if got := localOnly(c.addr, c.hosts); got != c.want {
			t.Errorf("localOnly(%q, %q) = %v, mau %v", c.addr, c.hosts, got, c.want)
		}
	}
}
