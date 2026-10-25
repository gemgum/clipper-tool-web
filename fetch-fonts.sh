#!/usr/bin/env bash
# Unduh font subtitle ke assets/fonts.
#
# Berdiri sendiri, dipanggil setup.sh dan langsung saat memasang di server.
# Font-nya sendiri tidak ikut di-commit — lihat .gitignore — jadi tanpa langkah
# ini folder itu kosong.
#
# Alamatnya HANYA ada di berkas ini. Ketika daftar font berubah, satu tempat
# yang disunting.
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

# Font preset caption (Word by word, Stack, Elastic, …; notes/45). Google hanya
# menyimpan versi VARIABEL dari kelimanya di repo, jadi yang diambil instans
# STATIS dari gstatic — family internalnya cocok dengan api.fontCatalog. Satu
# bobot saja tiap font: bobot yang dipakai presetnya.
st="https://fonts.gstatic.com/s"
dl "$st/inter/v20/UcCO3FwrK3iLTeHuS_nVMrMxCp50SjIw2boKoduKmMEVuFuYMZg.ttf" "$DEST/Inter-Bold.ttf"
dl "$st/figtree/v9/_Xmz-HUzqDCFdgfMsYiV_F7wfS-Bs_ehR15e.ttf" "$DEST/Figtree-SemiBold.ttf"
dl "$st/leaguespartan/v15/kJEnBuEW6A0lliaV_m88ja5Twtx8BWhtkDVmjZvM_oTpBA.ttf" "$DEST/LeagueSpartan.ttf"
dl "$st/poppins/v24/pxiByp8kv8JHgFVrLBT5V1s.ttf" "$DEST/Poppins-Black.ttf"
dl "$st/oswald/v57/TK3_WkUHHAIjg75cFRf3bXL8LICs1xZogUE.ttf" "$DEST/Oswald-Bold.ttf"
dl "$st/jetbrainsmono/v24/tDbY2o-flEEny0FZhsfKu5WU4zr3E_BX0PnT8RD8L6tjPQ.ttf" "$DEST/JetBrainsMono-Bold.ttf"
dl "$st/bangers/v25/FeVQS0BTqb0h60ACL5k.ttf" "$DEST/Bangers.ttf"
dl "$st/permanentmarker/v16/Fh4uPib9Iyv2ucM6pGQMWimMp004Hao.ttf" "$DEST/PermanentMarker.ttf"
dl "$st/titanone/v17/mFTzWbsGxbbS_J5cQcjykw.ttf" "$DEST/TitanOne.ttf"
dl "$st/bungee/v17/N0bU2SZBIuF2PU_ECg.ttf" "$DEST/Bungee.ttf"
dl "$st/pressstart2p/v16/e3t4euO8T-267oIAQAu6jDQyK0nS.ttf" "$DEST/PressStart2P.ttf"
dl "$st/pacifico/v23/FwZY7-Qmy14u9lezJ96A.ttf" "$DEST/Pacifico.ttf"
dl "$st/archivoblack/v23/HTxqL289NzCGg4MzN6KJ7eW6OYs.ttf" "$DEST/ArchivoBlack.ttf"
dl "$st/caveat/v23/WnznHAc5bAfYB2QRah7pcpNvOx-pjRV6SII.ttf" "$DEST/Caveat-Bold.ttf"

# Teks lisensinya IKUT, dan ini kewajiban bukan kesopanan: ketiga font berlisensi
# SIL Open Font License, dan OFL mensyaratkan salinan lisensinya menyertai font
# ketika fontnya disebarkan ulang, dan assets/fonts ikut disalin ke server apa
# adanya.
#
# Satu berkas per keluarga font, bukan satu gabungan: hak ciptanya berbeda
# pemilik, dan OFL menyertakan baris hak cipta itu di dalam teksnya.
dl "$gf/montserrat/OFL.txt" "$DEST/Montserrat-OFL.txt"
dl "$gf/anton/OFL.txt" "$DEST/Anton-OFL.txt"
dl "$gf/bebasneue/OFL.txt" "$DEST/BebasNeue-OFL.txt"
for f in inter figtree leaguespartan poppins oswald jetbrainsmono bangers titanone bungee pressstart2p pacifico archivoblack caveat; do
  dl "$gf/$f/OFL.txt" "$DEST/$f-OFL.txt"
done
# Permanent Marker berlisensi Apache 2.0, bukan OFL — teksnya di folder apache/.
dl "https://github.com/google/fonts/raw/main/apache/permanentmarker/LICENSE.txt" "$DEST/permanentmarker-LICENSE.txt"

# Nama family di dalam berkas font itulah yang ditulis ke .ass dan dicari
# libass; daftarnya ada di api.fontCatalog. Kalau salah satu gagal terunduh,
# subtitle diam-diam dirender dengan font pengganti — jadi lebih baik berhenti.
for f in Montserrat.ttf Montserrat-Regular.ttf Montserrat-Bold.ttf Anton.ttf BebasNeue.ttf \
         Inter-Bold.ttf Figtree-SemiBold.ttf LeagueSpartan.ttf Poppins-Black.ttf Oswald-Bold.ttf JetBrainsMono-Bold.ttf \
         Bangers.ttf PermanentMarker.ttf TitanOne.ttf Bungee.ttf PressStart2P.ttf Pacifico.ttf ArchivoBlack.ttf Caveat-Bold.ttf \
         Montserrat-OFL.txt Anton-OFL.txt BebasNeue-OFL.txt; do
  [ -s "$DEST/$f" ] || { echo "font $f gagal diunduh" >&2; exit 1; }
done
echo "    fonts -> $DEST"
