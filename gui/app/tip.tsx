"use client";

import { useRef, useState } from "react";
import { Info } from "lucide-react";

// Ikon ⓘ dengan keterangan (DESIGN.md §5.4). Dulu keterangannya cuma atribut
// `title` di label: tidak bisa dibuka lewat papan ketik dan tidak dibacakan
// sebagai deskripsi. Sekarang ikonnya tombol yang bisa difokus, dan kotaknya
// `position: fixed` — kolom halaman ini bergulir sendiri dan akan memotong
// kotak yang absolut (alasan yang sama dengan .popover).
export default function Tip({ text }: { text: string }) {
  const btn = useRef<HTMLButtonElement>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [pos, setPos] = useState<{ left: number; top: number; up: boolean } | null>(null);
  const id = useRef(`tip-${Math.random().toString(36).slice(2, 8)}`).current;

  const show = (delay: number) => {
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      const r = btn.current?.getBoundingClientRect();
      if (!r) return;
      const left = Math.min(Math.max(8, r.left + r.width / 2 - 120), window.innerWidth - 248);
      const up = r.bottom + 120 > window.innerHeight;
      setPos({ left, top: up ? r.top - 6 : r.bottom + 6, up });
    }, delay);
  };
  const hide = () => { if (timer.current) clearTimeout(timer.current); setPos(null); };

  return (
    <>
      <button ref={btn} type="button" className="tip" aria-label={text} aria-describedby={pos ? id : undefined}
        onMouseEnter={() => show(300)} onMouseLeave={hide} onFocus={() => show(0)} onBlur={hide}
        onKeyDown={(e) => { if (e.key === "Escape") hide(); }}
        onClick={(e) => e.preventDefault()}>
        <Info className="ico hint" aria-hidden="true" />
      </button>
      {pos && (
        <span id={id} role="tooltip" className={"tip-box" + (pos.up ? " up" : "")}
          style={{ left: pos.left, top: pos.top }}>{text}</span>
      )}
    </>
  );
}
