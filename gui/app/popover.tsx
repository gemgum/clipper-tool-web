"use client";

import { useCallback, useRef, useState } from "react";
import { usePopup } from "./use-popup";

// Popup yang MENEMPEL pada tombolnya. Satu komponen untuk semua — jelajah
// berita, palet warna, dan apa pun berikutnya.
//
// Dibuat jadi komponen bersama bukan karena rapi, melainkan karena satu
// detailnya mudah salah dan mahal: letaknya WAJIB `position: fixed`, bukan
// `absolute`. Tiap kolom di kerangka berzona ini punya `overflow-y: auto`
// sendiri, dan itu MEMOTONG anak yang diposisikan absolut begitu ia lebih lebar
// daripada kolomnya. Popup di sini hampir selalu lebih lebar (kisi tiga kolom,
// deretan contekan warna), jadi kesalahan itu pasti terjadi — bukan mungkin.
//
// Karena `fixed` tidak punya induk untuk diukur, letaknya dihitung dari
// getBoundingClientRect() tombolnya, lalu dijepit ke lebar jendela supaya tidak
// pernah keluar layar di jendela terkecil (900 px).
export default function Popover({
  label, ariaLabel, width = 320, align = "left", buttonClass = "ghost", disabled, onOpen,
  maxHeight = 460, side = "below", open: openProp, onOpenChange, children,
}: {
  label: React.ReactNode;
  /** Wajib bila label cuma ikon: tanpa ini pembaca layar mengumumkan "tombol" saja. */
  ariaLabel?: string;
  width?: number;
  align?: "left" | "right";
  buttonClass?: string;
  /** Batas atas tinggi popup; isinya bergulir di dalamnya. */
  maxHeight?: number;
  /** "beside" = keluar ke samping kanan tombolnya (tombol rail kiri). */
  side?: "below" | "beside";
  disabled?: boolean;
  onOpen?: () => void;
  /** Opsional: kendalikan dari luar, mis. tombol "Cari" yang harus membukanya. */
  open?: boolean;
  onOpenChange?: (v: boolean) => void;
  children: (close: () => void) => React.ReactNode;
}) {
  const [openLocal, setOpenLocal] = useState(false);
  const controlled = openProp !== undefined;
  const open = controlled ? openProp : openLocal;
  const setOpen = (v: boolean) => { if (!controlled) setOpenLocal(v); onOpenChange?.(v); };
  const btn = useRef<HTMLButtonElement>(null);
  const { pos, place } = usePopup({
    open, setOpen, anchor: btn, width, align, maxHeight, side,
    selectors: [".popover", ".popover-anchor"],
  });

  const close = useCallback(() => setOpen(false), [setOpen]);

  const toggle = () => {
    if (open) { setOpen(false); return; }
    place();
    setOpen(true);
    onOpen?.();
  };

  return (
    <div className="popover-anchor">
      <button ref={btn} type="button" disabled={disabled}
        className={buttonClass + (open ? " active" : "")}
        aria-label={ariaLabel} title={ariaLabel}
        aria-expanded={open} onClick={toggle}>
        {label}
      </button>
      {open && pos && (
        <div className={"popover" + (pos.up ? " up" : "")} style={{ top: pos.top, left: pos.left, width: pos.width, maxHeight: pos.maxH }}>
          {children(close)}
        </div>
      )}
    </div>
  );
}
