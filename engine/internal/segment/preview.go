package segment

import (
	"regexp"
	"strings"

	"github.com/gemgum/clipper/engine/internal/types"
)

// Cuplikan pembuka: banyak video dibuka dengan potongan-potongan pendek dari
// sepanjang video yang disambung jadi satu. Terdengar ramai — memang dirancang
// begitu — tapi berganti topik tiap beberapa detik dan tidak membahas apa pun
// sampai tuntas. Klip yang dipotong dari sana jadi trailer untuk video yang
// tidak akan ditonton siapa pun.
//
// Prompt saja tidak cukup. Diuji langsung: setelah aturan "jangan ambil dari
// cuplikan" ditambahkan ke prompt, qwen2.5 TETAP memilih momen 41-57 detik dari
// cuplikan. Model lokal tidak terikat instruksi panjang — sama seperti waktu ia
// membalas satu entri untuk 32 segmen sampai dipaksa JSON Schema.
//
// Yang membedakan cuplikan bukan ketidaknyambungannya. Itu sempat diukur dan
// GAGAL: kesinambungan kata antar-paruh di jendela cuplikan 0,22 dan 0,07,
// sementara banyak jendela isi justru 0,00. Percakapan spontan memang berpindah
// topik.
//
// Yang membedakannya: cuplikan adalah PRATAYANG. Hampir seluruh kosakatanya
// muncul lagi jauh di belakang, sebab ia memang dipotong dari sana. Terukur di
// tiga video:
//
//	cuplikan (video dengan cuplikan) : 0,97 dan 0,95
//	seluruh jendela isi, tiga video  : 0,33 - 0,81
//	90 detik pertama, dua video lain : 0,33 - 0,76 (memang tidak bercuplikan)
//
// # Kenapa yang dicari UJUNG cuplikan, bukan vonis per kandidat
//
// Versi pertama bertanya "apakah kandidat INI cuplikan?" dan lolos pada klip
// yang berkaki di dua tempat: 78,4-124,8 detik, separuh cuplikan separuh bumper
// acara ("Halo saudara semua, senang bisa menyapa..."). Kosakata bumper cuma
// diucapkan sekali seumur video, jadi ia MENGENCERKAN rasio gema klip itu jadi
// 0,607 — di dalam pita normal isi video (0,33-0,81).
//
// Artinya tidak ada ambang yang bisa memisahkannya. Bukan angkanya yang salah,
// tapi pertanyaannya. Maka yang dihitung sekarang: SAMPAI DETIK BERAPA cuplikan
// berakhir — sekali per transkrip — lalu kandidat mana pun yang mulai sebelum
// titik itu dibuang, seberapa pun jauh ekornya menjulur.
const (
	// previewThreshold = ambang "hampir semuanya muncul lagi nanti".
	//
	// 0,90 diambil di antara 0,81 (jendela isi tertinggi yang terukur) dan 0,95
	// (cuplikan terendah yang terukur), condong ke sisi aman: lebih baik satu
	// cuplikan lolos daripada satu momen isi yang bagus dibuang.
	previewThreshold = 0.90

	// previewOpening = sejauh mana dari awal video pemeriksaan ini berlaku.
	//
	// Cuplikan selalu di pembuka. Membatasi pemeriksaan ke sini membuat momen di
	// tengah video tidak mungkin salah tuduh, berapa pun kosakatanya bergema.
	previewOpening = 180.0

	// previewGap = jarak yang dianggap "jauh di belakang". Di bawah ini
	// pengulangan kata cuma tanda pembicaraan masih di topik yang sama.
	previewGap = 120.0

	// previewMinWords = kosakata terkecil yang layak dinilai. Di bawah ini satu
	// dua kata sudah menggeser rasionya jauh.
	previewMinWords = 8

	// previewWindow = lebar jendela pemindaian.
	//
	// Harus selebar ini karena sinyal PER SEGMEN terlalu berisik: di tengah
	// cuplikan pun ada segmen yang jatuh ke 0,75 hanya karena satu kata unik
	// ("tersangka", "terbetunya"). Diukur pada dua video bercuplikan, jendela 30
	// detik meratakan derau itu tanpa mengaburkan batasnya — rasio bertahan
	// 0,93-1,00 di sepanjang cuplikan lalu jatuh ke 0,58 tepat sesudahnya.
	previewWindow = 30.0

	// previewStep = jarak geser antar jendela. Lebih kecil = batas lebih tepat,
	// tapi tidak ada gunanya lebih halus daripada panjang satu-dua segmen.
	previewStep = 10.0
)

// reWord memilih kata yang cukup panjang untuk membawa makna. Kata pendek di
// bahasa Indonesia hampir semuanya kata tugas (yang, itu, dan, ke) — ia muncul
// di mana saja dan tidak menandakan topik apa pun.
var reWord = regexp.MustCompile(`[a-zA-Z]{6,}`)

func vocab(tr types.Transcript, from, to float64) map[string]bool {
	out := map[string]bool{}
	for _, s := range tr.Segments {
		if s.Start < from || s.End > to {
			continue
		}
		for _, w := range reWord.FindAllString(strings.ToLower(s.Text), -1) {
			out[w] = true
		}
	}
	return out
}

// echoRatio menghitung berapa bagian kosakata di [from,to] yang muncul lagi
// jauh di belakang. ok=false berarti tidak ada dasar untuk menilai — kosakatanya
// terlalu sedikit, atau tidak ada "nanti" untuk dibandingkan.
func echoRatio(tr types.Transcript, from, to float64) (float64, bool) {
	here := vocab(tr, from, to)
	if len(here) < previewMinWords {
		return 0, false
	}
	var last float64
	if n := len(tr.Segments); n > 0 {
		last = tr.Segments[n-1].End
	}
	later := vocab(tr, to+previewGap, last)
	if len(later) == 0 {
		return 0, false
	}
	echoed := 0
	for w := range here {
		if later[w] {
			echoed++
		}
	}
	return float64(echoed) / float64(len(here)), true
}

// OpeningPreviewEnd mengembalikan detik di mana cuplikan pembuka berakhir.
// Nol berarti video ini tidak dibuka dengan cuplikan.
//
// Dihitung SEKALI per transkrip, lalu dipakai untuk menyaring semua kandidat —
// bukan dihitung ulang per kandidat. Selain jauh lebih murah, itu juga yang
// membuat jawabannya konsisten: satu video punya satu ujung cuplikan, bukan
// satu vonis per klip.
//
// Jendela digeser dari detik 0 selama rasio gemanya masih tinggi. Begitu satu
// jendela jatuh — atau tidak bisa dinilai sama sekali — pemindaian berhenti di
// situ. Ujungnya diambil dari jendela TERAKHIR yang masih tinggi, jadi ekor
// cuplikan ikut terlindungi; wilayah tepat sesudahnya pun biasanya bumper
// acara, yang sama tidak layaknya untuk dijadikan klip.
func OpeningPreviewEnd(tr types.Transcript) float64 {
	end := 0.0
	for t := 0.0; t < previewOpening; t += previewStep {
		r, ok := echoRatio(tr, t, t+previewWindow)
		if !ok || r < previewThreshold {
			break
		}
		end = t + previewWindow
	}
	return end
}
