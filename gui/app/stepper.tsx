"use client";

// Ikon: lucide-react (ISC) — alasannya di gui/app/page.tsx. Tombolnya memakai
// ikon, BUKAN huruf "-" dan "+": tanda minus yang benar (U+2212) tidak ada di
// semua font, dan hubung ASCII terlihat lebih pendek daripada plusnya sehingga
// dua tombol sejajar tampak tidak sama.
import { Minus, Plus } from "lucide-react";
import { useI18n } from "./i18n";

// Angka dengan tombol −/+.
//
// Menggantikan <input type="range"> untuk Ukuran, Garis tepi, dan Zoom (6
// Agustus 2026). Penggeser melar selebar kolomnya, jadi tiga di antaranya
// membentang penuh dan panel terlihat berantakan padahal isinya cuma tiga
// angka. Penggeser juga tidak pernah menyebut nilainya sendiri — angkanya harus
// dititipkan ke label ("Size (72)"), dan nilai yang persis mustahil disetel
// dengan tetikus.
//
// Yang TETAP penggeser: waktu frame pratinjau. Itu menyusuri durasi video —
// mencari, bukan menyetel — dan di situ menyeret memang cara yang benar.
export default function Stepper({
  value, onChange, min, max, step = 1, suffix = "",
}: {
  value: number;
  onChange: (v: number) => void;
  min: number;
  max: number;
  step?: number;
  suffix?: string;
}) {
  const { t } = useI18n();
  const clamp = (v: number) => Math.min(max, Math.max(min, v));
  // Dibulatkan ke kelipatan step supaya angka yang diketik tangan tidak pernah
  // keluar dari nilai yang bisa dicapai tombolnya.
  const quantise = (v: number) => clamp(Math.round(v / step) * step);

  return (
    <div className="stepper">
      <button type="button" className="step-btn" aria-label={t("decrease")}
        disabled={value <= min} onClick={() => onChange(clamp(value - step))}>
        <Minus className="ico" aria-hidden="true" />
      </button>
      <input type="number" value={value} min={min} max={max} step={step}
        onChange={(e) => { const n = Number(e.target.value); if (!Number.isNaN(n)) onChange(quantise(n)); }} />
      {suffix && <span className="step-suffix">{suffix}</span>}
      <button type="button" className="step-btn" aria-label={t("increase")}
        disabled={value >= max} onClick={() => onChange(clamp(value + step))}>
        <Plus className="ico" aria-hidden="true" />
      </button>
    </div>
  );
}
