"use client";

import { useState } from "react";
import { ChevronDown } from "lucide-react";

// Kelompok setelan yang bisa diciutkan.
//
// Dipakai di kolom artikel halaman berita, dan alasannya bukan kerapian:
// isi kolom itu MUNCUL DAN HILANG mengikuti keadaan — kolom tautan hilang saat
// pindah ke jelajah, daftar paragraf muncul sesudah dianalisis, seluruh blok
// Isi baru ada setelah artikelnya termuat. Tiap kali itu terjadi, kelompok di
// bawahnya melompat, dan yang sedang dibaca pengguna pindah tempat.
//
// Menciutkan yang sudah selesai membuat kolomnya diam: sesudah artikel dipilih,
// bagian Artikel tidak perlu terbuka lagi.
//
// Judulnya <button>, bukan <div> dengan onClick: ia memang tombol, dan hanya
// dengan begitu ia bisa dijangkau Tab dan dibaca pembaca layar.
// open/onToggle membuatnya BOLEH dikendalikan dari luar. Diperlukan saat
// membuka satu kelompok berarti menutup kelompok lain — keadaan seperti itu
// tidak bisa tinggal di dalam salah satu dari mereka.
export default function Section({
  title, defaultOpen = true, open: openProp, onToggle, children,
}: {
  title: string;
  defaultOpen?: boolean;
  open?: boolean;
  onToggle?: (open: boolean) => void;
  children: React.ReactNode;
}) {
  const [openState, setOpenState] = useState(defaultOpen);
  const open = openProp ?? openState;
  const setOpen = () => (onToggle ? onToggle(!open) : setOpenState(!open));

  return (
    <div className="group">
      <button type="button" className="group-title group-toggle"
        aria-expanded={open} onClick={setOpen}>
        <span>{title}</span>
        <ChevronDown className={"ico chev" + (open ? " open" : "")} aria-hidden="true" />
      </button>
      {open && children}
    </div>
  );
}
