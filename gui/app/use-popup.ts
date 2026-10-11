"use client";

import { useCallback, useEffect, useState } from "react";

export type PopupPos = { top: number; left: number; width: number; maxH: number; up?: boolean };

// Letak & perilaku popup melayang, dipakai bersama <Popover> dan <Select>.
//
// Disatukan karena SATU aturannya mudah salah dan mahal: letaknya WAJIB
// `position: fixed`, bukan `absolute`. Tiap kolom di kerangka berzona ini punya
// `overflow-y: auto` sendiri, dan itu MEMOTONG anak yang diposisikan absolut
// begitu ia lebih lebar daripada kolomnya — dan isi popup di sini hampir selalu
// lebih lebar. Ditulis dua kali berarti dua tempat untuk salah.
//
// Karena `fixed` tidak punya induk untuk diukur, letaknya dihitung dari
// getBoundingClientRect() pemicunya lalu dijepit ke tepi jendela.
export function usePopup({
  open, setOpen, anchor, width, align = "left", selectors, maxHeight = 420, side = "below",
}: {
  open: boolean;
  setOpen: (v: boolean) => void;
  anchor: React.RefObject<HTMLElement | null>;
  /** Lebar yang diminta; dijepit ke lebar jendela. */
  width: number | ((rect: DOMRect) => number);
  align?: "left" | "right";
  /** Kelas yang dianggap "di dalam popup" saat memeriksa klik di luar. */
  selectors: [string, string];
  /** Batas atas tinggi popup; isinya bergulir sendiri di dalamnya. */
  maxHeight?: number;
  /**
   * "beside" = keluar ke SAMPING KANAN tombolnya, dasar sejajar dasar tombol.
   * Dipakai tombol di rail kiri: rail itu sempit dan berdiri di tepi jendela,
   * jadi popup yang membuka ke atas menutupi separuh halaman dan terlihat
   * melayang lepas dari tombol yang membukanya.
   */
  side?: "below" | "beside";
}) {
  const [pos, setPos] = useState<PopupPos | null>(null);

  const place = useCallback(() => {
    const r = anchor.current?.getBoundingClientRect();
    if (!r) return;
    const w = Math.min(typeof width === "function" ? width(r) : width, window.innerWidth - 32);
    if (side === "beside") {
      // Dasar popup sejajar dasar tombolnya, BERAPA PUN tingginya — itu yang
      // membuatnya terbaca sebagai milik tombol itu. Tingginya belum diketahui
      // saat ini dihitung, jadi dikerjakan CSS: top = dasar tombol, lalu
      // digeser naik setinggi dirinya sendiri (kelas "up").
      setPos({
        top: r.bottom,
        left: Math.min(r.right + 8, window.innerWidth - w - 8),
        width: w,
        maxH: Math.max(140, Math.min(maxHeight, r.bottom - 16)),
        up: true,
      });
      return;
    }
    const below = window.innerHeight - r.bottom - 12;
    const above = r.top - 12;
    // Membuka ke ATAS bila ruang di bawahnya lebih sempit. Ini WAJIB, bukan
    // kemewahan: tombol setelan/tema/akun berdiri di DASAR rail, jadi popup
    // yang selalu membuka ke bawah berakhir 282 px di bawah tepi layar —
    // terukur, dan artinya tidak terlihat sama sekali. Halaman tidak bisa
    // digulir untuk mengejarnya (body:has(.screen) overflow:hidden), jadi
    // popupnya benar-benar hilang.
    const down = below >= Math.min(240, above);
    const room = down ? below : above;
    const maxH = Math.max(140, Math.min(maxHeight, room));
    const wanted = align === "right" ? r.right - w : r.left;
    // Membuka ke atas: yang dipatok DASAR popup ke tepi atas tombol, lewat
    // kelas "up" (translateY(-100%) di CSS) — bukan `top = r.top - maxH`.
    //
    // Bedanya terukur dan pernah dilaporkan sebagai "error": maxH adalah tinggi
    // MAKSIMUM, sedangkan daftar berisi tiga pilihan cuma 103 px. Selisihnya
    // jadi jarak kosong, dan popupnya menggantung 183 px di atas tombolnya,
    // terlihat seperti milik panel lain. Daftar panjang kebetulan setinggi maxH,
    // jadi bug ini tidak pernah terlihat di sana.
    //
    // Tingginya baru diketahui setelah dirender, jadi ia tidak bisa dihitung di
    // sini — CSS yang mengerjakannya.
    setPos({
      top: down ? r.bottom + 6 : r.top - 6,
      left: Math.max(8, Math.min(wanted, window.innerWidth - w - 8)),
      width: w,
      maxH,
      up: !down,
    });
  }, [anchor, width, align, maxHeight, side]);

  // Esc, klik di luar, ubah ukuran, dan GULIR. Yang terakhir sering terlupa:
  // popup `fixed` tidak ikut bergerak bersama kolom yang digulir, jadi
  // membiarkannya terbuka berarti ia menggantung di tempat yang salah.
  useEffect(() => {
    if (!open) return;
    const [inPop, inAnchor] = selectors;
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") setOpen(false); };
    const onDown = (e: MouseEvent) => {
      const el = e.target as HTMLElement;
      if (!el.closest(inPop) && !el.closest(inAnchor)) setOpen(false);
    };
    // Gulir DI DALAM popup bukan alasan menutupnya — dan itu bukan kehalusan:
    // listener ini memakai capture, jadi menggulir daftar paragraf memicunya dan
    // popupnya menutup pada gerakan roda pertama. Yang harus menutup popup
    // hanyalah gulir yang MEMINDAHKAN tombolnya, yaitu gulir di luar popup.
    const onScroll = (e: Event) => {
      const el = e.target as HTMLElement | null;
      if (el?.closest?.(inPop)) return;
      setOpen(false);
    };
    window.addEventListener("keydown", onKey);
    window.addEventListener("mousedown", onDown);
    window.addEventListener("resize", place);
    window.addEventListener("scroll", onScroll, true);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", onScroll, true);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, place, setOpen]);

  // Dibuka dari luar (mis. tombol Cari) tidak melewati pemicunya, jadi letaknya
  // dihitung di sini juga — kalau tidak, popupnya muncul di koordinat lama.
  useEffect(() => { if (open) place(); }, [open, place]);

  return { pos, place };
}
