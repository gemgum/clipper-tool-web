package api

import (
	"encoding/binary"
	"fmt"
	"os"
)

// Berapa piksel CSS satu satuan "Fontsize" .ass — dan kenapa itu bukan 1.
//
// ASS mengartikan Fontsize sebagai TINGGI KOTAK font (winAscent + winDescent),
// sedangkan CSS mengartikan font-size sebagai ukuran em. Untuk Montserrat
// kotaknya 1379 per 1000 em, jadi "Fontsize 72" di .ass sama besarnya dengan
// "font-size 52,2px" di browser — pratinjau yang memakai 72px menampilkan teks
// 38% lebih besar daripada hasil rendernya.
//
// Diukur, bukan diturunkan dari teori: Fontsize 99 di libass menghasilkan lebar
// tinta yang sama persis dengan 72px di Chrome (484 vs 485 piksel), dan
// 72/99 = 0,727 sama dengan 1000/(1109+270) = 0,725 milik Montserrat.
//
// Angkanya BERBEDA tiap font — Anton 0,577, Bebas Neue 0,769 — jadi ini tidak
// bisa jadi satu tetapan di GUI. Engine yang punya berkasnya, jadi engine yang
// menghitung.
//
// Akibat lain dari definisi yang sama: jarak antarbaris libass persis sebesar
// Fontsize, berapa pun fontnya. Di CSS itu berarti line-height 1/scale.

// fontScale membaca berkas font dan mengembalikan piksel CSS per satuan Fontsize.
//
// Mengembalikan 0 bila berkasnya tidak bisa dibaca — pemanggil memperlakukannya
// sebagai "tidak tahu" dan tidak mengubah apa pun, sebab menebak di sini berarti
// pratinjau berbohong dengan percaya diri.
func fontScale(path string) float64 {
	upem, winAsc, winDesc, err := fontBox(path)
	if err != nil || upem == 0 || winAsc+winDesc == 0 {
		return 0
	}
	return float64(upem) / float64(int(winAsc)+int(winDesc))
}

// fontBox mengambil unitsPerEm dari tabel 'head' dan usWinAscent/usWinDescent
// dari tabel 'OS/2'.
//
// Ditulis tangan alih-alih memakai pustaka font: yang dibutuhkan cuma tiga
// angka pada offset tetap, dan proyek ini memakai pustaka standar saja.
func fontBox(path string) (upem, winAsc, winDesc uint16, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, 0, err
	}
	if len(raw) < 12 {
		return 0, 0, 0, fmt.Errorf("font file is too short")
	}
	// Koleksi (.ttc) tidak didukung: berkas font proyek ini selalu satu muka.
	numTables := int(binary.BigEndian.Uint16(raw[4:6]))
	if 12+16*numTables > len(raw) {
		return 0, 0, 0, fmt.Errorf("font table directory is out of range")
	}
	find := func(tag string) (int, bool) {
		for i := 0; i < numTables; i++ {
			e := 12 + 16*i
			if string(raw[e:e+4]) == tag {
				return int(binary.BigEndian.Uint32(raw[e+8 : e+12])), true
			}
		}
		return 0, false
	}

	head, ok := find("head")
	if !ok || head+20 > len(raw) {
		return 0, 0, 0, fmt.Errorf("font has no usable head table")
	}
	upem = binary.BigEndian.Uint16(raw[head+18 : head+20])

	// usWinAscent ada di offset 74 tabel OS/2, usWinDescent tepat sesudahnya.
	// Keduanya ada sejak versi 0, jadi versinya tidak perlu diperiksa.
	os2, ok := find("OS/2")
	if !ok || os2+78 > len(raw) {
		return 0, 0, 0, fmt.Errorf("font has no usable OS/2 table")
	}
	winAsc = binary.BigEndian.Uint16(raw[os2+74 : os2+76])
	winDesc = binary.BigEndian.Uint16(raw[os2+76 : os2+78])
	return upem, winAsc, winDesc, nil
}
