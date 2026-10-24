"use client";

// Crop bebas untuk foto lanjutan carousel (keputusan pemilik 25 Oktober 2026):
// gambar utuh di jendela, kotak potong digeser & ditarik di keempat sudutnya.
// Hasilnya pecahan gambar (0..1) + AR = rasio potongan dalam piksel asli —
// engine memakainya untuk memuat potongan utuh di kotak ilustrasi (card.Crop).

import { useEffect, useRef, useState } from "react";
import { useI18n } from "./i18n";

export type Crop = { x: number; y: number; w: number; h: number; ar: number };

const FULL = { x: 0, y: 0, w: 1, h: 1 };
const MIN = 0.05;

export default function CropDialog({ src, crop, onApply, onClose }: {
  src: string; crop?: Crop | null; onApply: (c: Crop | null) => void; onClose: () => void;
}) {
  const { t } = useI18n();
  const [r, setR] = useState(crop ? { x: crop.x, y: crop.y, w: crop.w, h: crop.h } : FULL);
  const [nat, setNat] = useState({ w: 1, h: 1 });
  const img = useRef<HTMLImageElement>(null);
  const grab = useRef<{ mode: string; px: number; py: number; r: typeof r } | null>(null);

  useEffect(() => {
    const esc = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", esc);
    return () => window.removeEventListener("keydown", esc);
  }, [onClose]);

  const clamp = (v: number, a: number, b: number) => Math.min(b, Math.max(a, v));
  const down = (mode: string) => (e: React.PointerEvent) => {
    e.stopPropagation();
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    grab.current = { mode, px: e.clientX, py: e.clientY, r };
  };
  const move = (e: React.PointerEvent) => {
    const g = grab.current, box = img.current?.getBoundingClientRect();
    if (!g || !box) return;
    const dx = (e.clientX - g.px) / box.width, dy = (e.clientY - g.py) / box.height;
    let { x, y, w, h } = g.r;
    if (g.mode === "move") {
      x = clamp(x + dx, 0, 1 - w); y = clamp(y + dy, 0, 1 - h);
    } else {
      // Sudut: n/s/e/w di nama mode menentukan tepi mana yang ikut tetikus.
      if (g.mode.includes("w")) { const nx = clamp(x + dx, 0, x + w - MIN); w += x - nx; x = nx; }
      if (g.mode.includes("e")) w = clamp(w + dx, MIN, 1 - x);
      if (g.mode.includes("n")) { const ny = clamp(y + dy, 0, y + h - MIN); h += y - ny; y = ny; }
      if (g.mode.includes("s")) h = clamp(h + dy, MIN, 1 - y);
    }
    setR({ x, y, w, h });
  };
  const up = () => { grab.current = null; };
  const full = r.x === 0 && r.y === 0 && r.w === 1 && r.h === 1;
  const pct = (v: number) => `${v * 100}%`;

  return (
    <div className="crop-modal" role="dialog" aria-label={t("cropTitle")} onClick={onClose}>
      <div className="crop-card" onClick={(e) => e.stopPropagation()}>
        <h2>{t("cropTitle")}</h2>
        <div className="crop-stage" onPointerMove={move} onPointerUp={up} onPointerCancel={up}>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img ref={img} src={src} alt="" draggable={false}
            onLoad={(e) => setNat({ w: e.currentTarget.naturalWidth || 1, h: e.currentTarget.naturalHeight || 1 })} />
          <div className="crop-rect" style={{ left: pct(r.x), top: pct(r.y), width: pct(r.w), height: pct(r.h) }} onPointerDown={down("move")}>
            {["nw", "ne", "sw", "se"].map((c) => <span key={c} className={"crop-h " + c} onPointerDown={down(c)} />)}
          </div>
        </div>
        <div className="crop-actions">
          <button type="button" className="ghost" disabled={full} onClick={() => setR(FULL)}>{t("cropReset")}</button>
          <span className="grow" />
          <button type="button" className="ghost" onClick={onClose}>{t("cancel")}</button>
          <button type="button" className="primary" onClick={() => onApply(full ? null : { ...r, ar: (r.w * nat.w) / (r.h * nat.h) })}>{t("cropApply")}</button>
        </div>
      </div>
    </div>
  );
}
