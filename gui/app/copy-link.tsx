"use client";

// copyText = SATU cara menyalin untuk seluruh aplikasi (Captions, Writer, News,
// kotak log). navigator.clipboard TIDAK ADA saat GUI dibuka lewat alamat IP
// (versi web lewat http) — bukan gagal, tapi undefined — jadi jalan
// cadangannya textarea + execCommand. Dulu tiap halaman memanggil
// navigator.clipboard langsung dan tombol Copy diam-diam mati di versi web.
export async function copyText(text: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
  } else {
    const area = document.createElement("textarea");
    area.value = text;
    area.style.position = "fixed";
    area.style.opacity = "0";
    document.body.appendChild(area);
    area.select();
    document.execCommand("copy");
    document.body.removeChild(area);
  }
}
