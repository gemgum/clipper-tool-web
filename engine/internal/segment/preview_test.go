package segment

import (
	"fmt"
	"testing"

	"github.com/gemgum/clipper/engine/internal/types"
)

// build menyusun transkrip dari daftar (mulai, teks) berdurasi 5 detik.
func build(lines []string, step float64) types.Transcript {
	tr := types.Transcript{Language: "id"}
	for i, s := range lines {
		tr.Segments = append(tr.Segments, types.TranscriptSegment{
			Start: float64(i) * step, End: float64(i)*step + step - 0.1, Text: s,
		})
	}
	return tr
}

// Cuplikan = pratayang: hampir seluruh kosakatanya muncul lagi jauh di belakang,
// sebab ia memang dipotong dari sana.
func TestOpeningPreviewIsRecognised(t *testing.T) {
	lines := []string{
		"pemerintah membangun universitas kebangsaan",
		"koruptor ditangkap kejaksaan minggu kemarin",
		"wartawan menuliskan berita meninggalnya penjaga",
	}
	// Isi video mengulang seluruhnya, jauh setelah cuplikan.
	for i := 0; i < 40; i++ {
		lines = append(lines, "percakapan biasa tentang keseharian bersama keluarga")
	}
	lines = append(lines,
		"pemerintah membangun universitas kebangsaan itu keliru sekali",
		"koruptor ditangkap kejaksaan minggu kemarin memang benar",
		"wartawan menuliskan berita meninggalnya penjaga rumahnya",
	)
	tr := build(lines, 5)
	if OpeningPreviewEnd(tr) <= 0 {
		t.Error("cuplikan pembuka tidak dikenali")
	}
}

// Pembuka yang memang isi — kosakatanya tidak bergema di belakang — harus lolos.
func TestRealOpeningIsKept(t *testing.T) {
	lines := []string{
		"selamat datang pembahasan konstitusi negara republik",
		"konstitusi mengatur hubungan pemerintah dengan rakyatnya",
		"perjanjian masyarakat melahirkan kewenangan bernegara",
	}
	for i := 0; i < 40; i++ {
		lines = append(lines, fmt.Sprintf("pembicaraan lainnya seputar perkara berbeda nomor %d puluhan", i))
	}
	tr := build(lines, 5)
	if end := OpeningPreviewEnd(tr); end > 0 {
		t.Errorf("pembuka yang memang isi ikut dibuang (ujung cuplikan %.0f)", end)
	}
}

// Pemeriksaan hanya berlaku di pembuka. Momen di tengah video tidak boleh
// tertuduh, berapa pun kosakatanya bergema — di sana pengulangan kata justru
// tanda pembicaraan masih di topik yang sama.
func TestOnlyTheOpeningIsChecked(t *testing.T) {
	// Kosakatanya harus cukup banyak (lihat previewMinWords) supaya yang diuji
	// benar-benar batas posisinya, bukan tersangkut penjaga jumlah kata.
	var lines []string
	for i := 0; i < 120; i++ {
		lines = append(lines, "pembahasan konstitusi kebangsaan tentang kewenangan pemerintah "+
			"bersama masyarakat menjalankan perjanjian kehidupan berbangsa")
	}
	tr := build(lines, 5) // 600 detik
	end := OpeningPreviewEnd(tr)
	if end <= 0 {
		t.Fatal("prasyarat uji tidak terpenuhi: pembuka seharusnya terdeteksi bergema")
	}
	// Walau SELURUH video bergema, vonisnya tidak boleh menjalar melewati
	// pembuka — kalau tidak, video yang topiknya konsisten akan kehilangan
	// separuh isinya.
	if end > previewOpening+previewWindow {
		t.Errorf("ujung cuplikan %.0f menjalar melewati pembuka (%.0f)", end, previewOpening)
	}
}

// Rentang tanpa kosakata yang cukup tidak boleh dinilai: satu dua kata sudah
// menggeser rasionya jauh, dan menuduh berdasarkan itu asal-asalan.
func TestTooFewWordsIsNeverFlagged(t *testing.T) {
	tr := build([]string{"iya", "oh ya", "begitu"}, 5)
	if OpeningPreviewEnd(tr) > 0 {
		t.Error("rentang tanpa kata isi ikut dituduh cuplikan")
	}
}

// Tidak ada "nanti" untuk dibandingkan → tidak ada dasar menuduh.
func TestNothingAfterMeansNoVerdict(t *testing.T) {
	tr := build([]string{
		"pemerintah membangun universitas kebangsaan bersama",
		"koruptor ditangkap kejaksaan kemarin sekali",
	}, 5)
	if OpeningPreviewEnd(tr) > 0 {
		t.Error("dituduh cuplikan padahal tidak ada isi setelahnya")
	}
}

// hybridTranscript meniru bentuk yang dulu lolos: cuplikan, lalu bumper acara
// yang kosakatanya diucapkan SEKALI seumur video, lalu isi sebenarnya.
//
//	0-90    cuplikan  (kosakatanya bergema di isi)
//	90-125  bumper    (kosakatanya unik, tidak pernah muncul lagi)
//	125+    isi
func hybridTranscript() types.Transcript {
	teaser := []string{
		"pemerintah membangun universitas kebangsaan bersama masyarakat",
		"koruptor ditangkap kejaksaan kemarin dengan tuduhan penggelapan",
		"wartawan menuliskan berita meninggalnya penjaga rumahnya",
	}
	var lines []string
	for i := 0; i < 6; i++ {
		lines = append(lines, teaser...) // 18 baris = 90 detik
	}
	lines = append(lines, // bumper: kosakata yang tidak muncul di mana pun lagi
		"halo pemirsa selamat menyaksikan tayangan mingguan",
		"jangan lupa berlangganan menekan lonceng notifikasi",
		"sekarang mari simak obrolannya sampai penghabisan",
		"disiarkan langsung setiap petang menjelang",
		"terima kasih sudah menemani sepanjang episodenya",
		"salam hangat kepada seluruh penonton setianya",
		"ikuti juga tayangan berikutnya pekan mendatang",
	)
	for i := 0; i < 45; i++ { // isi: mengulang kosakata cuplikan
		lines = append(lines, teaser[i%len(teaser)])
	}
	return build(lines, 5)
}

// Inilah bug yang sebenarnya: klip 78,4-124,8 detik berkaki di dua tempat —
// separuh cuplikan, separuh bumper. Kosakata bumper mengencerkan rasio gemanya
// jadi 0,607, di dalam pita normal isi video, sehingga vonis per kandidat tidak
// mungkin menangkapnya. Yang menangkapnya: mengetahui di mana cuplikan berakhir.
func TestStraddlingClipIsCaught(t *testing.T) {
	tr := hybridTranscript()
	end := OpeningPreviewEnd(tr)
	if end <= 0 {
		t.Fatal("cuplikan tidak dikenali sama sekali")
	}
	// Klip yang MULAI di dalam cuplikan tapi berakhir jauh di dalam bumper.
	const straddleStart = 75.0
	if straddleStart >= end {
		t.Errorf("klip berkaki dua (mulai %.0f) lolos: ujung cuplikan cuma %.0f",
			straddleStart, end)
	}
	// Dan ujungnya tidak boleh menjalar sampai ke isi.
	if end >= 125 {
		t.Errorf("ujung cuplikan %.0f sudah memakan isi video (mulai 125)", end)
	}
}

// Rasio klip berkaki dua itu memang jatuh di dalam pita isi video — bukti bahwa
// tidak ada ambang yang bisa memisahkannya, dan karena itu pendekatan lamanya
// mustahil diperbaiki dengan menyetel angka.
func TestStraddlingClipRatioSitsInsideTheContentBand(t *testing.T) {
	r, ok := echoRatio(hybridTranscript(), 75, 125)
	if !ok {
		t.Fatal("rasio tidak terhitung")
	}
	if r >= previewThreshold {
		t.Fatalf("prasyarat uji meleset: rasio %.2f, seharusnya di bawah ambang %.2f", r, previewThreshold)
	}
	t.Logf("rasio klip berkaki dua = %.2f (ambang %.2f) — persis kenapa vonis per kandidat gagal",
		r, previewThreshold)
}
