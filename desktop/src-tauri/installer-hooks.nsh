; Uninstaller: tawarkan membuang data pengguna juga.
;
; Uninstaller bawaan hanya tahu folder programnya (Program Files / AppData
; Local\<identifier>). Data Clipper TIDAK di sana: `config.userDataDir` menaruh
; semuanya di %LOCALAPPDATA%\Clipper — model whisper (bisa 3 GB), whisper.cpp &
; ffmpeg yang diunduh halaman Requirements, cache transkrip, riwayat job, .env,
; serta klip & kartu yang tersimpan di folder bawaan. Tanpa berkas ini, uninstall
; menyisakan berkilo-kilobyte sampai gigabytes yang tidak pernah disebut siapa
; pun (dilaporkan 7 Agustus 2026).
;
; DITAWARKAN, bukan dipaksa: memasang ulang versi baru jauh lebih enak kalau
; model 3 GB-nya tidak perlu diunduh lagi. Tombol bawaannya "No" — di antara
; "menyisakan berkas" dan "menghapus klip orang", yang kedua jauh lebih mahal.

!macro NSIS_HOOK_POSTUNINSTALL
  ; Uninstall senyap (mis. dijalankan perkakas manajemen) tidak boleh membuang
  ; data: tidak ada yang bisa menjawab, dan diamnya bukan berarti setuju.
  IfSilent clipper_keep_data

  IfFileExists "$LOCALAPPDATA\Clipper\*.*" 0 clipper_keep_data
  MessageBox MB_YESNO|MB_ICONEXCLAMATION|MB_DEFBUTTON2 \
    "Also delete Clipper's data folder?$\n$\n\
$LOCALAPPDATA\Clipper$\n$\n\
This removes the whisper models, the downloaded tools, the transcript cache, \
the job history, and any clips or cards still saved in the default folder.$\n$\n\
Choose No to keep them for a future install." \
    IDNO clipper_keep_data
  RMDir /r "$LOCALAPPDATA\Clipper"

  clipper_keep_data:
!macroend
