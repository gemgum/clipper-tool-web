package job

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Log job DI DISK: <DataDir>/jobs/<id>.log, bersebelahan dengan catatan JSON-nya.
//
// Ada karena kotak log di GUI hidup sebagai state React. Berpindah tab melepas
// komponen halamannya dan seluruh isinya ikut mati, dan tidak ada satu pun
// tempat lain yang menyimpannya: Job cuma memegang Stage & Progress, dan
// Subscribe tidak pernah memutar ulang peristiwa yang sudah lewat.
//
// Berkas, bukan penyangga di memori. Berkas bisa dibaca berapa kali pun dengan
// hasil yang sama, selamat dari halaman yang dimuat ulang, selamat dari
// aplikasi yang ditutup, dan bisa dilampirkan apa adanya ke laporan bug — hal
// yang sekarang mustahil, sebab satu-satunya salinan mati di dalam array React.
//
// Isinya SELALU bahasa Inggris: ini catatan teknis, dan berkas yang isinya
// berubah menurut pilihan bahasa justru menyulitkan saat dilampirkan. Pesan
// yang lahir di GUI (kemajuan unggahan, hasil cek font) TIDAK ikut — yang
// berharga saat job gagal adalah tahap yang dikerjakan engine.
//
// Satu berkas per job, sama seperti catatan JSON-nya: ia mati bersama jobnya,
// jadi tidak ada rotasi dan tidak ada batas ukuran yang perlu dijaga.
func (m *Manager) logPath(id string) string {
	return filepath.Join(m.jobsDir(), id+".log")
}

// logf menambahkan satu baris bertimestamp.
func (m *Manager) logf(id, format string, args ...any) {
	m.write(id, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...)))
}

// logRaw menambahkan teks apa adanya, tanpa cap waktu.
//
// Dipakai tabel ringkasan waktu per tahap: ia berisi banyak baris yang
// perataannya harus utuh, dan cap waktu di depan baris pertama saja justru
// merusak kolomnya.
func (m *Manager) logRaw(id, text string) { m.write(id, text) }

// write menambahkan satu potong ke berkas log. Kegagalannya diabaikan dengan
// sengaja: baris log yang hilang menyebalkan, job yang batal dirender karena
// disk penuh jauh lebih buruk.
func (m *Manager) write(id, text string) {
	if os.MkdirAll(m.jobsDir(), 0o755) != nil {
		return
	}
	f, err := os.OpenFile(m.logPath(id), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, text)
}

// ReadLog mengembalikan seluruh baris log satu job. Job tanpa berkas log
// (mis. riwayat dari versi sebelum ini) mengembalikan daftar kosong, bukan
// galat: halaman yang menampilkannya tidak boleh gagal karenanya.
func (m *Manager) ReadLog(id string) []string {
	b, err := os.ReadFile(m.logPath(id))
	if err != nil {
		return []string{}
	}
	trimmed := strings.TrimRight(string(b), "\n")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "\n")
}
