#!/usr/bin/env bash
# Unduh font subtitle ke assets/fonts.
#
# Berdiri sendiri, dipanggil dua tempat: setup.sh (mesin pengembang) dan alur
# GitHub Actions (mesin pembangun pemasang). Font-nya sendiri tidak ikut
# di-commit — lihat .gitignore — jadi tanpa langkah ini folder itu kosong, dan
# pengemasan Tauri berhenti dengan "resource path assets\fonts doesn't exist".
#
# Alamatnya HANYA ada di berkas ini. Ketika daftar font berubah, satu tempat
# yang disunting; dulu daftar ini ada di setup.sh dan tidak terlihat oleh alur
# CI sama sekali.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
DEST="${1:-$ROOT/assets/fonts}"

mkdir -p "$DEST"
gf="https://github.com/google/fonts/raw/main/ofl"

# dl mengunduh bila belum ada. Sudah ada = lewati: font tidak berubah, dan
# mengunduh ulang tiap build hanya menambah satu titik gagal.
dl() {
  if [ -f "$2" ]; then
    echo "    ada: $(basename "$2")"
    return
  fi
  curl -sL --fail --retry 3 "$1" -o "$2"
  echo "    unduh: $(basename "$2")"
}

# Montserrat diunduh DUA KALI dalam bentuk berbeda, dan itu disengaja:
#
#   Montserrat.ttf           font VARIABEL (wght 100..900), untuk kartu berita.
#                            Kartu dirender Chrome, yang memahami sumbu variabel,
#                            jadi satu berkas melayani seluruh bobot (card.go
#                            memasangnya dengan font-weight:100 900).
#   Montserrat-Regular.ttf   face STATIS, untuk subtitle video.
#   Montserrat-Bold.ttf
#
# Kenapa subtitle tidak boleh memakai yang variabel — ini diukur, bukan dikira:
# berkas variabel Google menyebut dirinya family "Montserrat THIN", sedangkan
# engine menulis "Montserrat" ke .ass. libass karena itu tidak mengenalinya sama
# sekali dan jatuh ke font sistem: render dengan berkas itu di fontsdir IDENTIK
# byte demi byte dengan render tanpa font sama sekali.
#
# Kenapa harus DUA face statis, bukan satu: dengan Regular saja, Bold=1 hanya
# menghasilkan penebalan buatan yang jauh lebih tipis daripada Bold sungguhan
# (3.755 vs 6.056 piksel tinta pada uji yang sama). Dengan Bold saja, pilihan
# "bold: off" berhenti berfungsi. Dua face membuat keduanya benar.
#
# JANGAN memakai Montserrat-SemiBold: family di dalamnya "Montserrat SemiBold",
# jadi ia tidak akan pernah cocok dengan "Montserrat" — persoalan yang sama
# dengan yang variabel.
dl "$gf/montserrat/Montserrat%5Bwght%5D.ttf" "$DEST/Montserrat.ttf"

# Statisnya diambil dari repo hulu (Google Fonts hanya menyimpan yang variabel),
# dipaku ke satu tag supaya hasilnya bisa diulang.
ms="https://github.com/JulietaUla/Montserrat/raw/v7.222/fonts/ttf"
dl "$ms/Montserrat-Regular.ttf" "$DEST/Montserrat-Regular.ttf"
dl "$ms/Montserrat-Bold.ttf" "$DEST/Montserrat-Bold.ttf"

dl "$gf/anton/Anton-Regular.ttf" "$DEST/Anton.ttf"
dl "$gf/bebasneue/BebasNeue-Regular.ttf" "$DEST/BebasNeue.ttf"

# Teks lisensinya IKUT, dan ini kewajiban bukan kesopanan: ketiga font berlisensi
# SIL Open Font License, dan OFL mensyaratkan salinan lisensinya menyertai font
# ketika fontnya disebarkan ulang. Pemasang Windows membundel assets/fonts apa
# adanya (build-windows.sh), jadi tanpa berkas ini font orang lain terkirim ke
# pengguna tanpa lisensinya.
#
# Satu berkas per keluarga font, bukan satu gabungan: hak ciptanya berbeda
# pemilik, dan OFL menyertakan baris hak cipta itu di dalam teksnya.
dl "$gf/montserrat/OFL.txt" "$DEST/Montserrat-OFL.txt"
dl "$gf/anton/OFL.txt" "$DEST/Anton-OFL.txt"
dl "$gf/bebasneue/OFL.txt" "$DEST/BebasNeue-OFL.txt"

# Nama family di dalam berkas font itulah yang ditulis ke .ass dan dicari
# libass; daftarnya ada di api.fontCatalog. Kalau salah satu gagal terunduh,
# subtitle diam-diam dirender dengan font pengganti — jadi lebih baik berhenti.
for f in Montserrat.ttf Montserrat-Regular.ttf Montserrat-Bold.ttf Anton.ttf BebasNeue.ttf \
         Montserrat-OFL.txt Anton-OFL.txt BebasNeue-OFL.txt; do
  [ -s "$DEST/$f" ] || { echo "font $f gagal diunduh" >&2; exit 1; }
done
echo "    fonts -> $DEST"
